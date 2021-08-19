package client_test

import (
	"context"
	"crypto/rand"
	"math"
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
	email := apitest.NewEmail()
	id, err := c.CreateCustomer(context.Background(), key, email, apitest.NewUsername(), mdb.Dev)
	require.NoError(t, err)

	childKey := newKey(t)
	childId, err := c.CreateCustomer(
		context.Background(),
		childKey,
		apitest.NewEmail(),
		apitest.NewUsername(),
		mdb.User,
		client.WithParent(key, email, mdb.Dev),
	)
	require.NoError(t, err)

	err = c.UpdateCustomer(context.Background(), id, 100, true, true)
	require.NoError(t, err)

	cus, err := c.GetCustomer(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, 100, int(cus.Balance))
	assert.True(t, cus.Billable)
	assert.True(t, cus.Delinquent)

	// Child cannot be billable
	err = c.UpdateCustomer(context.Background(), childId, 0, true, false)
	require.Error(t, err)
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
	var inc, sizeTotal, unitsTotal int64

	err = c.UpdateCustomer(context.Background(), id, 0, true, false)
	require.NoError(t, err)

	inc = product.FreeQuotaSize * 2
	sizeTotal += inc
	unitsTotal += getUnits(inc, product)
	_, err = c.IncCustomerUsage(context.Background(), key, map[string]int64{test.key: inc})
	require.NoError(t, err)

	err = c.ReportCustomerUsage(context.Background(), key)
	require.NoError(t, err)

	res, err := c.GetCustomerUsage(context.Background(), key)
	require.NoError(t, err)
	assert.NotEmpty(t, res.Usage)
	assert.Equal(t, sizeTotal, res.Usage[test.key].Total)
	assert.Equal(t, unitsTotal, res.Usage[test.key].Units)
	assert.Equal(t, getCost(unitsTotal, product, test.unitPrice), res.Usage[test.key].Cost)
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

	product := getProduct(t, test.key)
	var inc, sizeTotal, unitsTotal int64

	// Add some under unit size
	inc = test.initialIncSize
	sizeTotal += inc
	unitsTotal += getUnits(inc, product)
	res, err := c.IncCustomerUsage(context.Background(), key, map[string]int64{test.key: inc})
	require.NoError(t, err)
	assert.Equal(t, sizeTotal, res.DailyUsage[test.key].Total)
	assert.Equal(t, unitsTotal, res.DailyUsage[test.key].Units)
	assert.Equal(t, getCost(unitsTotal, product, test.unitPrice), res.DailyUsage[test.key].Cost)

	// Add more to reach unit size
	inc = product.UnitSize - test.initialIncSize
	sizeTotal += inc
	unitsTotal += getUnits(inc, product)
	res, err = c.IncCustomerUsage(context.Background(), key, map[string]int64{test.key: inc})
	require.NoError(t, err)
	assert.Equal(t, sizeTotal, res.DailyUsage[test.key].Total)
	assert.Equal(t, unitsTotal, res.DailyUsage[test.key].Units)
	assert.Equal(t, getCost(unitsTotal, product, test.unitPrice), res.DailyUsage[test.key].Cost)

	// Add a bunch of units above free quota (should error)
	inc = product.FreeQuotaSize
	sizeTotal += inc
	unitsTotal += getUnits(inc, product)
	res, err = c.IncCustomerUsage(context.Background(), key, map[string]int64{test.key: inc})
	require.Error(t, err)

	// Add some more as a child (should error since parent is above free quota)
	// Child should only see their usage, but it should also be accrued on the parent
	childInc := product.UnitSize
	childSizeTotal := childInc
	childUnitsTotal := getUnits(childInc, product)
	inc = childInc
	sizeTotal += inc
	unitsTotal += getUnits(inc, product)
	res, err = c.IncCustomerUsage(context.Background(), childKey, map[string]int64{test.key: childInc})
	require.Error(t, err)

	// Flag parent as billable to remove the free quota limit
	err = c.UpdateCustomer(context.Background(), id, 0, true, false)
	require.NoError(t, err)

	// Try again
	inc = product.FreeQuotaSize
	sizeTotal += inc
	unitsTotal += getUnits(inc, product)
	res, err = c.IncCustomerUsage(context.Background(), key, map[string]int64{test.key: inc})
	require.NoError(t, err)
	assert.Equal(t, sizeTotal, res.DailyUsage[test.key].Total)
	assert.Equal(t, unitsTotal, res.DailyUsage[test.key].Units)
	assert.Equal(t, getCost(unitsTotal, product, test.unitPrice), res.DailyUsage[test.key].Cost)

	// Try again as a child
	childInc = product.UnitSize
	childSizeTotal += childInc
	childUnitsTotal += getUnits(childInc, product)
	inc = childInc
	sizeTotal += inc
	unitsTotal += getUnits(inc, product)
	res, err = c.IncCustomerUsage(context.Background(), childKey, map[string]int64{test.key: childInc})
	require.NoError(t, err)
	assert.Equal(t, childSizeTotal, res.DailyUsage[test.key].Total)
	assert.Equal(t, childUnitsTotal, res.DailyUsage[test.key].Units)
	assert.Equal(t, getCost(childUnitsTotal, product, test.unitPrice), res.DailyUsage[test.key].Cost)

	// Bump child to over _their_ free quota limit, since parent is billing, they should not see an error
	childInc = product.FreeQuotaSize
	childSizeTotal += childInc
	childUnitsTotal += getUnits(childInc, product)
	inc = childInc
	sizeTotal += inc
	unitsTotal += getUnits(inc, product)
	res, err = c.IncCustomerUsage(context.Background(), childKey, map[string]int64{test.key: childInc})
	require.NoError(t, err)
	assert.Equal(t, childSizeTotal, res.DailyUsage[test.key].Total)
	assert.Equal(t, childUnitsTotal, res.DailyUsage[test.key].Units)
	assert.Equal(t, getCost(childUnitsTotal, product, test.unitPrice), res.DailyUsage[test.key].Cost)

	// Check total usage
	cus, err := c.GetCustomer(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, sizeTotal, cus.DailyUsage[test.key].Total)
	assert.Equal(t, unitsTotal, cus.DailyUsage[test.key].Units)
	assert.Equal(t, getCost(unitsTotal, product, test.unitPrice), cus.DailyUsage[test.key].Cost)
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

func getUnits(size int64, product *service.Product) int64 {
	return int64(math.Round(float64(size) / float64(product.UnitSize)))
}

func getCost(units int64, product *service.Product, price float64) float64 {
	paidUnits := units - getUnits(product.FreeQuotaSize, product)
	if paidUnits < 0 {
		return 0
	}
	return float64(paidUnits) * price
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
