package client_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stripe "github.com/stripe/stripe-go/v72"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/v2/api/apitest"
	"github.com/textileio/textile/v2/api/billingd/client"
	"github.com/textileio/textile/v2/api/billingd/service"
	"github.com/textileio/textile/v2/util"
	"google.golang.org/grpc"
)

const (
	mib = 1024 * 1024
)

func TestClient_CheckHealth(t *testing.T) {
	t.Parallel()
	c := setup(t)
	err := c.CheckHealth(context.Background())
	require.NoError(t, err)
}

func TestClient_CreateCustomer(t *testing.T) {
	t.Parallel()
	c := setup(t)

	key := newKey(t)
	_, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	_, err = c.CreateCustomer(context.Background(), newKey(t), client.WithEmail(apitest.NewEmail()))
	require.NoError(t, err)

	_, err = c.CreateCustomer(context.Background(), newKey(t), client.WithParentKey(newKey(t)))
	require.Error(t, err) // Parent does not exist
	_, err = c.CreateCustomer(context.Background(), newKey(t), client.WithParentKey(key))
	require.NoError(t, err)
}

func TestClient_GetCustomer(t *testing.T) {
	t.Parallel()
	c := setup(t)
	key := newKey(t)
	_, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	cus, err := c.GetCustomer(context.Background(), key)
	require.NoError(t, err)
	assert.NotEmpty(t, cus.AccountStatus)
	assert.NotEmpty(t, cus.SubscriptionStatus)
	assert.Equal(t, 0, int(cus.Balance))
	assert.False(t, cus.Billable)
	assert.False(t, cus.Delinquent)
	assert.NotEmpty(t, cus.DailyUsage)
}

func TestClient_GetCustomerSession(t *testing.T) {
	t.Parallel()
	c := setup(t)
	key := newKey(t)
	_, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	session, err := c.GetCustomerSession(context.Background(), key)
	require.NoError(t, err)
	assert.NotEmpty(t, session.Url)
}

func TestClient_ListDependentCustomers(t *testing.T) {
	t.Parallel()
	c := setup(t)
	key := newKey(t)
	_, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	for i := 0; i < 30; i++ {
		_, err = c.CreateCustomer(context.Background(), newKey(t), client.WithParentKey(key))
		require.NoError(t, err)
		time.Sleep(time.Second)
	}

	res, err := c.ListDependentCustomers(context.Background(), key, client.WithLimit(30))
	require.NoError(t, err)
	assert.Len(t, res.Customers, 30)

	res, err = c.ListDependentCustomers(context.Background(), key)
	require.NoError(t, err)
	assert.Len(t, res.Customers, 25)

	res, err = c.ListDependentCustomers(context.Background(), key, client.WithLimit(5))
	require.NoError(t, err)
	assert.Len(t, res.Customers, 5)

	res, err = c.ListDependentCustomers(context.Background(), key, client.WithOffset(res.NextOffset))
	require.NoError(t, err)
	assert.Len(t, res.Customers, 25)

	res, err = c.ListDependentCustomers(context.Background(), key, client.WithOffset(res.NextOffset))
	require.NoError(t, err)
	assert.Len(t, res.Customers, 0)
}

func TestClient_UpdateCustomer(t *testing.T) {
	t.Parallel()
	c := setup(t)
	key := newKey(t)
	id, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	err = c.UpdateCustomer(context.Background(), id, 100, true, true)
	require.NoError(t, err)

	cus, err := c.GetCustomer(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, 100, int(cus.Balance))
	assert.True(t, cus.Billable)
	assert.True(t, cus.Delinquent)
}

func TestClient_UpdateCustomerSubscription(t *testing.T) {
	t.Parallel()
	c := setup(t)
	key := newKey(t)
	id, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	start := time.Now().Add(-time.Hour).Unix()
	end := time.Now().Add(time.Hour).Unix()
	err = c.UpdateCustomerSubscription(context.Background(), id, stripe.SubscriptionStatusCanceled, start, end)
	require.NoError(t, err)

	cus, err := c.GetCustomer(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, string(stripe.SubscriptionStatusCanceled), cus.SubscriptionStatus)
}

func TestClient_RecreateCustomerSubscription(t *testing.T) {
	t.Parallel()
	c := setup(t)
	key := newKey(t)
	id, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	err = c.RecreateCustomerSubscription(context.Background(), key)
	require.Error(t, err)

	start := time.Now().Add(-time.Hour).Unix()
	end := time.Now().Add(time.Hour).Unix()
	err = c.UpdateCustomerSubscription(context.Background(), id, stripe.SubscriptionStatusCanceled, start, end)
	require.NoError(t, err)

	err = c.RecreateCustomerSubscription(context.Background(), key)
	require.NoError(t, err)

	cus, err := c.GetCustomer(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, string(stripe.SubscriptionStatusActive), cus.SubscriptionStatus)
}

func TestClient_DeleteCustomer(t *testing.T) {
	t.Parallel()
	c := setup(t)
	key := newKey(t)
	_, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	err = c.DeleteCustomer(context.Background(), key)
	require.NoError(t, err)
}

type usageTest struct {
	key            string
	initialIncSize int64
	unitPrice      float64
}

func TestClient_IncCustomerUsage(t *testing.T) {
	t.Parallel()
	tests := []usageTest{
		{"stored_data", mib, 0.000007705471},
		{"network_egress", mib, 0.000025684903},
		{"instance_reads", 1, 0.000099999999},
		{"instance_writes", 1, 0.000199999999},
	}
	for _, test := range tests {
		incCustomerUsage(t, test)
	}
}

func incCustomerUsage(t *testing.T, test usageTest) {
	c := setup(t)
	key := newKey(t)
	id, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	product := getProduct(t, test.key)
	freeUnitsPerInterval := getFreeUnitsPerInterval(product)

	// Add some under unit size
	res, err := c.IncCustomerUsage(context.Background(), key, map[string]int64{test.key: test.initialIncSize})
	require.NoError(t, err)
	assert.Equal(t, int64(0), res.DailyUsage[test.key].Units)
	assert.Equal(t, test.initialIncSize, res.DailyUsage[test.key].Total)
	assert.Equal(t, float64(0), res.DailyUsage[test.key].Cost)

	// Add more to reach unit size
	res, err = c.IncCustomerUsage(context.Background(), key, map[string]int64{test.key: product.UnitSize - test.initialIncSize})
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.DailyUsage[test.key].Units)
	assert.Equal(t, product.UnitSize, res.DailyUsage[test.key].Total)
	assert.Equal(t, float64(0), res.DailyUsage[test.key].Cost)

	// Add a bunch of units above free quota
	res, err = c.IncCustomerUsage(context.Background(), key, map[string]int64{test.key: product.FreeQuotaSize})
	require.Error(t, err)

	// Flag as billable to remove the free quota limit
	err = c.UpdateCustomer(context.Background(), id, 0, true, false)
	require.NoError(t, err)

	// Try again
	res, err = c.IncCustomerUsage(context.Background(), key, map[string]int64{test.key: product.FreeQuotaSize})
	require.NoError(t, err)
	assert.Equal(t, freeUnitsPerInterval+1, res.DailyUsage[test.key].Units)
	assert.Equal(t, product.FreeQuotaSize+product.UnitSize, res.DailyUsage[test.key].Total)
	assert.Equal(t, test.unitPrice, res.DailyUsage[test.key].Cost)

	// Try as a child customer
	childKey := newKey(t)
	_, err = c.CreateCustomer(context.Background(), childKey, client.WithParentKey(key))
	require.NoError(t, err)
	res, err = c.IncCustomerUsage(context.Background(), childKey, map[string]int64{test.key: product.UnitSize})
	require.NoError(t, err)
	assert.Equal(t, int64(1), res.DailyUsage[test.key].Units)
	assert.Equal(t, product.UnitSize, res.DailyUsage[test.key].Total)
	assert.Equal(t, float64(0), res.DailyUsage[test.key].Cost)

	// Check total usage
	cus, err := c.GetCustomer(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, freeUnitsPerInterval+2, cus.DailyUsage[test.key].Units)
	assert.Equal(t, product.FreeQuotaSize+(2*product.UnitSize), cus.DailyUsage[test.key].Total)
	assert.Equal(t, test.unitPrice*2, cus.DailyUsage[test.key].Cost)
}

func getProduct(t *testing.T, key string) *service.Product {
	for _, p := range service.Products {
		if p.Key == key {
			return &p
		}
	}
	t.Fatalf("could not find product with key %s", key)
	return nil
}

func getFreeUnitsPerInterval(product *service.Product) int64 {
	return product.FreeQuotaSize / product.UnitSize
}

func setup(t *testing.T) *client.Client {
	apiPort, err := freeport.GetFreePort()
	require.NoError(t, err)
	gwPort, err := freeport.GetFreePort()
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := service.NewService(ctx, service.Config{
		ListenAddr:             util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", apiPort)),
		StripeAPIURL:           "https://api.stripe.com",
		StripeAPIKey:           os.Getenv("STRIPE_API_KEY"),
		StripeSessionReturnURL: "http://127.0.0.1:8006/dashboard",
		DBURI:           "mongodb://127.0.0.1:27017",
		DBName:          util.MakeToken(8),
		GatewayHostAddr: util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gwPort)),
		Debug:           true,
	})
	require.NoError(t, err)
	err = api.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		err := api.Stop(true)
		require.NoError(t, err)
	})

	c, err := client.NewClient(fmt.Sprintf("127.0.0.1:%d", apiPort), grpc.WithInsecure())
	require.NoError(t, err)

	t.Cleanup(func() {
		err := c.Close()
		require.NoError(t, err)
	})
	return c
}

func newKey(t *testing.T) thread.PubKey {
	_, key, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	return thread.NewLibp2pPubKey(key)
}
