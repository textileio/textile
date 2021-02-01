package client_test

import (
	"context"
	"crypto/rand"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stripe "github.com/stripe/stripe-go/v72"
	"github.com/textileio/go-ds-mongo/test"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/apitest"
	"github.com/textileio/textile/v2/api/billingd/client"
	"github.com/textileio/textile/v2/api/billingd/service"
	mdb "github.com/textileio/textile/v2/mongodb"
	"google.golang.org/grpc"
)

const (
	mib = 1024 * 1024
)

func TestMain(m *testing.M) {
	cleanup := func() {}
	if os.Getenv("SKIP_SERVICES") != "true" {
		cleanup = test.StartMongoDB()
	}
	exitVal := m.Run()
	cleanup()
	os.Exit(exitVal)
}

func TestClient_CheckHealth(t *testing.T) {
	c := setup(t)
	err := c.CheckHealth(context.Background())
	require.NoError(t, err)
}

func TestClient_CreateCustomer(t *testing.T) {
	c := setup(t)

	key := newKey(t)
	email := apitest.NewEmail()
	username := apitest.NewUsername()
	_, err := c.CreateCustomer(context.Background(), key, email, username, mdb.Dev)
	require.NoError(t, err)

	_, err = c.CreateCustomer(context.Background(), key, email, username, mdb.Dev)
	require.Error(t, err)

	_, err = c.CreateCustomer(
		context.Background(),
		newKey(t),
		apitest.NewEmail(),
		apitest.NewUsername(),
		mdb.User,
		client.WithParent(key, email, mdb.Dev),
	)
	require.NoError(t, err)

	nonExistentParentKey := newKey(t)
	_, err = c.CreateCustomer(
		context.Background(),
		newKey(t),
		apitest.NewEmail(),
		apitest.NewUsername(),
		mdb.User,
		client.WithParent(nonExistentParentKey, apitest.NewEmail(), mdb.Dev),
	)
	require.NoError(t, err)

	newParent, err := c.GetCustomer(context.Background(), nonExistentParentKey)
	require.NoError(t, err)
	assert.NotEmpty(t, newParent)
	assert.Equal(t, int64(1), newParent.Dependents)
}

func TestClient_GetCustomer(t *testing.T) {
	c := setup(t)
	key := newKey(t)
	_, err := c.CreateCustomer(context.Background(), key, apitest.NewEmail(), apitest.NewUsername(), mdb.Dev)
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
	c := setup(t)
	key := newKey(t)
	_, err := c.CreateCustomer(context.Background(), key, apitest.NewEmail(), apitest.NewUsername(), mdb.Dev)
	require.NoError(t, err)

	session, err := c.GetCustomerSession(context.Background(), key)
	require.NoError(t, err)
	assert.NotEmpty(t, session.Url)
}

func TestClient_ListDependentCustomers(t *testing.T) {
	c := setup(t)
	key := newKey(t)
	email := apitest.NewEmail()
	username := apitest.NewUsername()
	_, err := c.CreateCustomer(context.Background(), key, email, username, mdb.Org)
	require.NoError(t, err)

	for i := 0; i < 30; i++ {
		_, err = c.CreateCustomer(
			context.Background(),
			newKey(t),
			apitest.NewEmail(),
			apitest.NewUsername(),
			mdb.User,
			client.WithParent(key, email, mdb.Org),
		)
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
	c := setup(t)
	key := newKey(t)
	id, err := c.CreateCustomer(context.Background(), key, apitest.NewEmail(), apitest.NewUsername(), mdb.Dev)
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
	c := setup(t)
	key := newKey(t)
	id, err := c.CreateCustomer(context.Background(), key, apitest.NewEmail(), apitest.NewUsername(), mdb.Dev)
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
	c := setup(t)
	key := newKey(t)
	id, err := c.CreateCustomer(context.Background(), key, apitest.NewEmail(), apitest.NewUsername(), mdb.Dev)
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
	c := setup(t)
	key := newKey(t)
	_, err := c.CreateCustomer(context.Background(), key, apitest.NewEmail(), apitest.NewUsername(), mdb.Dev)
	require.NoError(t, err)

	err = c.DeleteCustomer(context.Background(), key)
	require.NoError(t, err)
}

type usageTest struct {
	key            string
	initialIncSize int64
	unitPrice      float64
}

func TestClient_GetCustomerUsage(t *testing.T) {
	tests := []usageTest{
		{"stored_data", mib, 0.000007705471},
		{"network_egress", mib, 0.000025684903},
		{"instance_reads", 1, 0.000099999999},
		{"instance_writes", 1, 0.000199999999},
	}
	for _, tt := range tests {
		getCustomerUsage(t, tt)
	}
}

func getCustomerUsage(t *testing.T, test usageTest) {
	c := setup(t)
	key := newKey(t)
	id, err := c.CreateCustomer(context.Background(), key, apitest.NewEmail(), apitest.NewUsername(), mdb.Dev)
	require.NoError(t, err)

	product := getProduct(t, test.key)

	err = c.UpdateCustomer(context.Background(), id, 0, true, false)
	require.NoError(t, err)

	_, err = c.IncCustomerUsage(context.Background(), key, map[string]int64{test.key: product.FreeQuotaSize * 2})
	require.NoError(t, err)

	err = c.ReportCustomerUsage(context.Background(), key)
	require.NoError(t, err)

	res, err := c.GetCustomerUsage(context.Background(), key)
	require.NoError(t, err)
	assert.NotEmpty(t, res.Usage)
	assert.Equal(t, product.FreeQuotaSize*2, res.Usage[test.key].Total)
	assert.Equal(t, float64(product.FreeQuotaSize/product.UnitSize)*test.unitPrice, res.Usage[test.key].Cost)
}

func TestClient_IncCustomerUsage(t *testing.T) {
	tests := []usageTest{
		{"stored_data", mib, 0.000007705471},
		{"network_egress", mib, 0.000025684903},
		{"instance_reads", 1, 0.000099999999},
		{"instance_writes", 1, 0.000199999999},
	}
	for _, tt := range tests {
		incCustomerUsage(t, tt)
	}
}

func incCustomerUsage(t *testing.T, test usageTest) {
	c := setup(t)
	key := newKey(t)
	email := apitest.NewEmail()
	username := apitest.NewUsername()
	id, err := c.CreateCustomer(context.Background(), key, email, username, mdb.Dev)
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
	_, err = c.CreateCustomer(
		context.Background(),
		childKey,
		apitest.NewEmail(),
		apitest.NewUsername(),
		mdb.User,
		client.WithParent(key, email, mdb.Dev),
	)
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
	bconf := apitest.DefaultBillingConfig(t)
	bconf.FreeQuotaGracePeriod = 0
	bconf.DBURI = test.GetMongoUri()
	apitest.MakeBillingWithConfig(t, bconf)

	billingApi, err := tutil.TCPAddrFromMultiAddr(bconf.ListenAddr)
	require.NoError(t, err)
	c, err := client.NewClient(billingApi, grpc.WithInsecure())
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
