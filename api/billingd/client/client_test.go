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

	_, err = c.CreateCustomer(context.Background(), key, client.WithParentKey(newKey(t)))
	require.Error(t, err)
	_, err = c.CreateCustomer(context.Background(), key, client.WithParentKey(newKey(t)))
	require.Error(t, err)
}

func TestClient_GetCustomer(t *testing.T) {
	t.Parallel()
	c := setup(t)
	key := newKey(t)
	_, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	cus, err := c.GetCustomer(context.Background(), key)
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
	key := newKey(t)
	_, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	session, err := c.GetCustomerSession(context.Background(), key)
	require.NoError(t, err)
	assert.NotEmpty(t, session.Url)
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
	assert.Equal(t, string(stripe.SubscriptionStatusCanceled), cus.Status)
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
	assert.Equal(t, string(stripe.SubscriptionStatusActive), cus.Status)
}

func TestClient_IncStoredData(t *testing.T) {
	t.Parallel()
	c := setup(t)
	key := newKey(t)
	id, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	// Add some under unit size
	res, err := c.IncStoredData(context.Background(), key, mib)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.StoredData.Units))
	assert.Equal(t, mib, int(res.StoredData.Total))

	// Add more to reach unit size
	res, err = c.IncStoredData(context.Background(), key, service.StoredDataUnitSize-mib)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.StoredData.Units))
	assert.Equal(t, service.StoredDataUnitSize, int(res.StoredData.Total))

	// Add a bunch of units above free quota
	res, err = c.IncStoredData(context.Background(), key, service.StoredDataFreePerInterval)
	require.Error(t, err)

	// Flag as billable to remove the free quota limit
	err = c.UpdateCustomer(context.Background(), id, 0, true, false)
	require.NoError(t, err)

	// Try again
	res, err = c.IncStoredData(context.Background(), key, service.StoredDataFreePerInterval)
	require.NoError(t, err)
	assert.Equal(t, service.StoredDataFreeUnitsPerInterval+1, int(res.StoredData.Units))
	assert.Equal(t, service.StoredDataFreePerInterval+service.StoredDataUnitSize, int(res.StoredData.Total))

	// Try as a child customer
	childKey := newKey(t)
	_, err = c.CreateCustomer(context.Background(), childKey, client.WithParentKey(key))
	require.NoError(t, err)
	res, err = c.IncStoredData(context.Background(), childKey, service.StoredDataUnitSize)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.StoredData.Units))
	assert.Equal(t, service.StoredDataUnitSize, int(res.StoredData.Total))

	// Check total usage
	cus, err := c.GetCustomer(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, service.StoredDataFreeUnitsPerInterval+2, int(cus.StoredData.Units))
	assert.Equal(t, service.StoredDataFreePerInterval+(2*service.StoredDataUnitSize), int(cus.StoredData.Total))
}

func TestClient_IncNetworkEgress(t *testing.T) {
	t.Parallel()
	c := setup(t)
	key := newKey(t)
	id, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	// Add some under unit size
	res, err := c.IncNetworkEgress(context.Background(), key, mib)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.NetworkEgress.Units))
	assert.Equal(t, mib, int(res.NetworkEgress.Total))

	// Add more to reach unit size
	res, err = c.IncNetworkEgress(context.Background(), key, service.NetworkEgressUnitSize-mib)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.NetworkEgress.Units))
	assert.Equal(t, service.NetworkEgressUnitSize, int(res.NetworkEgress.Total))

	// Add a bunch of units above free quota
	res, err = c.IncNetworkEgress(context.Background(), key, service.NetworkEgressFreePerInterval)
	require.Error(t, err)

	// Flag as billable to remove the free quota limit
	err = c.UpdateCustomer(context.Background(), id, 0, true, false)
	require.NoError(t, err)

	// Try again
	res, err = c.IncNetworkEgress(context.Background(), key, service.NetworkEgressFreePerInterval)
	require.NoError(t, err)
	assert.Equal(t, service.NetworkEgressFreeUnitsPerInterval+1, int(res.NetworkEgress.Units))
	assert.Equal(t, service.NetworkEgressFreePerInterval+service.NetworkEgressUnitSize, int(res.NetworkEgress.Total))

	// Try as a child customer
	childKey := newKey(t)
	_, err = c.CreateCustomer(context.Background(), childKey, client.WithParentKey(key))
	require.NoError(t, err)
	res, err = c.IncNetworkEgress(context.Background(), childKey, service.NetworkEgressUnitSize)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.NetworkEgress.Units))
	assert.Equal(t, service.NetworkEgressUnitSize, int(res.NetworkEgress.Total))

	// Check total usage
	cus, err := c.GetCustomer(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, service.NetworkEgressFreeUnitsPerInterval+2, int(cus.NetworkEgress.Units))
	assert.Equal(t, service.NetworkEgressFreePerInterval+(2*service.NetworkEgressUnitSize), int(cus.NetworkEgress.Total))
}

func TestClient_IncInstanceReads(t *testing.T) {
	t.Parallel()
	c := setup(t)
	key := newKey(t)
	id, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	// Add some under unit size
	res, err := c.IncInstanceReads(context.Background(), key, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.InstanceReads.Units))
	assert.Equal(t, 1, int(res.InstanceReads.Total))

	// Add more to reach unit size
	res, err = c.IncInstanceReads(context.Background(), key, service.InstanceReadsUnitSize-1)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.InstanceReads.Units))
	assert.Equal(t, service.InstanceReadsUnitSize, int(res.InstanceReads.Total))

	// Add a bunch of units above free quota
	res, err = c.IncInstanceReads(context.Background(), key, service.InstanceReadsFreePerInterval)
	require.Error(t, err)

	// Flag as billable to remove the free quota limit
	err = c.UpdateCustomer(context.Background(), id, 0, true, false)
	require.NoError(t, err)

	// Try again
	res, err = c.IncInstanceReads(context.Background(), key, service.InstanceReadsFreePerInterval)
	require.NoError(t, err)
	assert.Equal(t, service.InstanceReadsFreeUnitsPerInterval+1, int(res.InstanceReads.Units))
	assert.Equal(t, service.InstanceReadsFreePerInterval+service.InstanceReadsUnitSize, int(res.InstanceReads.Total))

	// Try as a child customer
	childKey := newKey(t)
	_, err = c.CreateCustomer(context.Background(), childKey, client.WithParentKey(key))
	require.NoError(t, err)
	res, err = c.IncInstanceReads(context.Background(), childKey, service.InstanceReadsUnitSize)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.InstanceReads.Units))
	assert.Equal(t, service.InstanceReadsUnitSize, int(res.InstanceReads.Total))

	// Check total usage
	cus, err := c.GetCustomer(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, service.InstanceReadsFreeUnitsPerInterval+2, int(cus.InstanceReads.Units))
	assert.Equal(t, service.InstanceReadsFreePerInterval+(2*service.InstanceReadsUnitSize), int(cus.InstanceReads.Total))
}

func TestClient_IncInstanceWrites(t *testing.T) {
	t.Parallel()
	c := setup(t)
	key := newKey(t)
	id, err := c.CreateCustomer(context.Background(), key)
	require.NoError(t, err)

	// Add some under unit size
	res, err := c.IncInstanceWrites(context.Background(), key, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.InstanceWrites.Units))
	assert.Equal(t, 1, int(res.InstanceWrites.Total))

	// Add more to reach unit size
	res, err = c.IncInstanceWrites(context.Background(), key, service.InstanceWritesUnitSize-1)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.InstanceWrites.Units))
	assert.Equal(t, service.InstanceWritesUnitSize, int(res.InstanceWrites.Total))

	// Add a bunch of units above free quota
	res, err = c.IncInstanceWrites(context.Background(), key, service.InstanceWritesFreePerInterval)
	require.Error(t, err)

	// Add a card to remove the free quota limit
	err = c.UpdateCustomer(context.Background(), id, 0, true, false)
	require.NoError(t, err)

	// Try again
	res, err = c.IncInstanceWrites(context.Background(), key, service.InstanceWritesFreePerInterval)
	require.NoError(t, err)
	assert.Equal(t, service.InstanceWritesFreeUnitsPerInterval+1, int(res.InstanceWrites.Units))
	assert.Equal(t, service.InstanceWritesFreePerInterval+service.InstanceWritesUnitSize, int(res.InstanceWrites.Total))

	// Try as a child customer
	childKey := newKey(t)
	_, err = c.CreateCustomer(context.Background(), childKey, client.WithParentKey(key))
	require.NoError(t, err)
	res, err = c.IncInstanceWrites(context.Background(), childKey, service.InstanceWritesUnitSize)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.InstanceWrites.Units))
	assert.Equal(t, service.InstanceWritesUnitSize, int(res.InstanceWrites.Total))

	// Check total usage
	cus, err := c.GetCustomer(context.Background(), key)
	require.NoError(t, err)
	assert.Equal(t, service.InstanceWritesFreeUnitsPerInterval+2, int(cus.InstanceWrites.Units))
	assert.Equal(t, service.InstanceWritesFreePerInterval+(2*service.InstanceWritesUnitSize), int(cus.InstanceWrites.Total))
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
