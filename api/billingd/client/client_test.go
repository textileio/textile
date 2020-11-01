package client_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	stripe "github.com/stripe/stripe-go/v72"
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
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)
	assert.NotEmpty(t, id)
}

func TestClient_GetCustomer(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	cus, err := c.GetCustomer(context.Background(), id)
	require.NoError(t, err)
	assert.NotEmpty(t, cus.Status)
	assert.Equal(t, 0, int(cus.Balance))
	assert.False(t, cus.Billable)
	assert.False(t, cus.Delinquent)
	assert.NotEmpty(t, cus.StoredData)
	assert.NotEmpty(t, cus.NetworkEgress)
	assert.NotEmpty(t, cus.InstanceReads)
	assert.NotEmpty(t, cus.InstanceWrites)
}

func TestClient_GetCustomerSession(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	session, err := c.GetCustomerSession(context.Background(), id)
	require.NoError(t, err)
	assert.NotEmpty(t, session.Url)
}

func TestClient_DeleteCustomer(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	err = c.DeleteCustomer(context.Background(), id)
	require.NoError(t, err)
}

func TestClient_UpdateCustomer(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	err = c.UpdateCustomer(context.Background(), id, 100, true, true)
	require.NoError(t, err)

	cus, err := c.GetCustomer(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 100, int(cus.Balance))
	assert.True(t, cus.Billable)
	assert.True(t, cus.Delinquent)
}

func TestClient_UpdateCustomerSubscription(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	start := time.Now().Add(-time.Hour).Unix()
	end := time.Now().Add(time.Hour).Unix()
	err = c.UpdateCustomerSubscription(context.Background(), id, stripe.SubscriptionStatusCanceled, start, end)
	require.NoError(t, err)

	cus, err := c.GetCustomer(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, string(stripe.SubscriptionStatusCanceled), cus.Status)
}

func TestClient_RecreateCustomerSubscription(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	err = c.RecreateCustomerSubscription(context.Background(), id)
	require.Error(t, err)

	start := time.Now().Add(-time.Hour).Unix()
	end := time.Now().Add(time.Hour).Unix()
	err = c.UpdateCustomerSubscription(context.Background(), id, stripe.SubscriptionStatusCanceled, start, end)
	require.NoError(t, err)

	err = c.RecreateCustomerSubscription(context.Background(), id)
	require.NoError(t, err)

	cus, err := c.GetCustomer(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, string(stripe.SubscriptionStatusActive), cus.Status)
}

func TestClient_IncStoredData(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Units should round down
	res, err := c.IncStoredData(context.Background(), id, mib)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.StoredData.Units))
	assert.Equal(t, mib, int(res.StoredData.Total))

	// Units should round up
	res, err = c.IncStoredData(context.Background(), id, 30*mib)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.StoredData.Units))
	assert.Equal(t, 31*mib, int(res.StoredData.Total))

	// Units should round down
	res, err = c.IncStoredData(context.Background(), id, 289*mib)
	require.NoError(t, err)
	assert.Equal(t, 6, int(res.StoredData.Units))
	assert.Equal(t, 320*mib, int(res.StoredData.Total))

	// Check total usage
	cus, err := c.GetCustomer(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 6, int(cus.StoredData.Units))
	assert.Equal(t, 320*mib, int(cus.StoredData.Total))
}

func TestClient_IncNetworkEgress(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Add some under unit size
	res, err := c.IncNetworkEgress(context.Background(), id, mib)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.NetworkEgress.Units))
	total := mib
	assert.Equal(t, total, int(res.NetworkEgress.Total))

	// Add more to reach unit size
	res, err = c.IncNetworkEgress(context.Background(), id, service.NetworkEgressUnitSize-mib)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.NetworkEgress.Units))
	total += service.NetworkEgressUnitSize - mib
	assert.Equal(t, total, int(res.NetworkEgress.Total))

	// Add a bunch of units above free quota
	res, err = c.IncNetworkEgress(context.Background(), id, service.NetworkEgressUnitSize*200+mib)
	require.Error(t, err)

	// Flag as billable to remove the free quota limit
	err = c.UpdateCustomer(context.Background(), id, 0, true, false)
	require.NoError(t, err)

	// Try again
	res, err = c.IncNetworkEgress(context.Background(), id, service.NetworkEgressUnitSize*200+mib)
	require.NoError(t, err)
	assert.Equal(t, 201, int(res.NetworkEgress.Units))
	total += service.NetworkEgressUnitSize*200 + mib
	assert.Equal(t, total, int(res.NetworkEgress.Total))

	// Check total usage
	cus, err := c.GetCustomer(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 201, int(cus.NetworkEgress.Units))
	assert.Equal(t, total, int(cus.NetworkEgress.Total))
}

func TestClient_IncInstanceReads(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Add some under unit size
	res, err := c.IncInstanceReads(context.Background(), id, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.InstanceReads.Units))
	assert.Equal(t, 1, int(res.InstanceReads.Total))

	// Add more to reach unit size
	res, err = c.IncInstanceReads(context.Background(), id, 9999)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.InstanceReads.Units))
	assert.Equal(t, 10000, int(res.InstanceReads.Total))

	// Add a bunch of units above free quota
	res, err = c.IncInstanceReads(context.Background(), id, 123456)
	require.Error(t, err)

	// Flag as billable to remove the free quota limit
	err = c.UpdateCustomer(context.Background(), id, 0, true, false)
	require.NoError(t, err)

	// Try again
	res, err = c.IncInstanceReads(context.Background(), id, 123456)
	require.NoError(t, err)
	assert.Equal(t, 13, int(res.InstanceReads.Units))
	assert.Equal(t, 133456, int(res.InstanceReads.Total))

	// Check total usage
	cus, err := c.GetCustomer(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 13, int(cus.InstanceReads.Units))
	assert.Equal(t, 133456, int(cus.InstanceReads.Total))
}

func TestClient_IncInstanceWrites(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Add some under unit size
	res, err := c.IncInstanceWrites(context.Background(), id, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.InstanceWrites.Units))
	assert.Equal(t, 1, int(res.InstanceWrites.Total))

	// Add more to reach unit size
	res, err = c.IncInstanceWrites(context.Background(), id, 4999)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.InstanceWrites.Units))
	assert.Equal(t, 5000, int(res.InstanceWrites.Total))

	// Add a bunch of units above free quota
	res, err = c.IncInstanceWrites(context.Background(), id, 123456)
	require.Error(t, err)

	// Add a card to remove the free quota limit
	err = c.UpdateCustomer(context.Background(), id, 0, true, false)
	require.NoError(t, err)

	// Try again
	res, err = c.IncInstanceWrites(context.Background(), id, 123456)
	require.NoError(t, err)
	assert.Equal(t, 26, int(res.InstanceWrites.Units))
	assert.Equal(t, 128456, int(res.InstanceWrites.Total))

	// Check total usage
	cus, err := c.GetCustomer(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 26, int(cus.InstanceWrites.Units))
	assert.Equal(t, 128456, int(cus.InstanceWrites.Total))
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
		StripeAPIKey:           "sk_test_RuU6Lq65WP23ykDSI9N9nRbC",
		StripeSessionReturnURL: "http://127.0.0.1:8006/dashboard",
		DBURI:           "mongodb://127.0.0.1:27017",
		DBName:          util.MakeToken(8),
		GatewayHostAddr: util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gwPort)),
		Debug:           true,
	}, true)
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
