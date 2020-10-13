package client_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.Equal(t, 0, int(cus.Balance))
	assert.False(t, cus.Billable)
	assert.False(t, cus.Delinquent)
}

func TestClient_DeleteCustomer(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	err = c.DeleteCustomer(context.Background(), id)
	require.NoError(t, err)
}

func TestClient_AddCard(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	err = c.AddCard(context.Background(), id, apitest.NewCardToken(t))
	require.NoError(t, err)

	cus, err := c.GetCustomer(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 0, int(cus.Balance))
	assert.True(t, cus.Billable)
	assert.False(t, cus.Delinquent)
}

func TestClient_SetStoredData(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Units should round down
	res, err := c.SetStoredData(context.Background(), id, mib)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.Units))
	assert.False(t, res.UnitsChanged)

	// Units should round up
	res, err = c.SetStoredData(context.Background(), id, 30*mib)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.Units))
	assert.True(t, res.UnitsChanged)

	// Units should round down
	res, err = c.SetStoredData(context.Background(), id, 260*mib)
	require.NoError(t, err)
	assert.Equal(t, 5, int(res.Units))
	assert.True(t, res.UnitsChanged)

	// Check total usage
	usage, err := c.GetStoredData(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 5, int(usage.Units))
	assert.Equal(t, 260*mib, int(usage.TotalSize))
}

func TestClient_IncNetworkEgress(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Add some under unit size
	res, err := c.IncNetworkEgress(context.Background(), id, mib)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.AddedUnits))
	assert.Equal(t, mib, int(res.SubUnits))

	// Add more to reach unit size
	res, err = c.IncNetworkEgress(context.Background(), id, service.NetworkEgressUnitSize-mib)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.AddedUnits))
	assert.Equal(t, 0, int(res.SubUnits))

	// Add a bunch of units above free quota
	res, err = c.IncNetworkEgress(context.Background(), id, service.NetworkEgressUnitSize*200+mib)
	require.Error(t, err)

	// Add a card to remove the free quota limit
	err = c.AddCard(context.Background(), id, apitest.NewCardToken(t))
	require.NoError(t, err)

	// Try again
	res, err = c.IncNetworkEgress(context.Background(), id, service.NetworkEgressUnitSize*200+mib)
	require.NoError(t, err)
	assert.Equal(t, 200, int(res.AddedUnits))
	assert.Equal(t, mib, int(res.SubUnits))

	// Check total usage
	usage, err := c.GetNetworkEgress(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 201, int(usage.Units))
	assert.Equal(t, mib, int(usage.SubUnits))
}

func TestClient_IncInstanceReads(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Add some under unit size
	res, err := c.IncInstanceReads(context.Background(), id, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.AddedUnits))
	assert.Equal(t, 1, int(res.SubUnits))

	// Add more to reach unit size
	res, err = c.IncInstanceReads(context.Background(), id, 9999)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.AddedUnits))
	assert.Equal(t, 0, int(res.SubUnits))

	// Add a bunch of units above free quota
	res, err = c.IncInstanceReads(context.Background(), id, 123456)
	require.Error(t, err)

	// Add a card to remove the free quota limit
	err = c.AddCard(context.Background(), id, apitest.NewCardToken(t))
	require.NoError(t, err)

	// Try again
	res, err = c.IncInstanceReads(context.Background(), id, 123456)
	require.NoError(t, err)
	assert.Equal(t, 12, int(res.AddedUnits))
	assert.Equal(t, 3456, int(res.SubUnits))

	// Check total usage
	usage, err := c.GetInstanceReads(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 13, int(usage.Units))
	assert.Equal(t, 3456, int(usage.SubUnits))
}

func TestClient_IncInstanceWrites(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Add some under unit size
	res, err := c.IncInstanceWrites(context.Background(), id, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.AddedUnits))
	assert.Equal(t, 1, int(res.SubUnits))

	// Add more to reach unit size
	res, err = c.IncInstanceWrites(context.Background(), id, 4999)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.AddedUnits))
	assert.Equal(t, 0, int(res.SubUnits))

	// Add a bunch of units above free quota
	res, err = c.IncInstanceWrites(context.Background(), id, 123456)
	require.Error(t, err)

	// Add a card to remove the free quota limit
	err = c.AddCard(context.Background(), id, apitest.NewCardToken(t))
	require.NoError(t, err)

	// Try again
	res, err = c.IncInstanceWrites(context.Background(), id, 123456)
	require.NoError(t, err)
	assert.Equal(t, 24, int(res.AddedUnits))
	assert.Equal(t, 3456, int(res.SubUnits))

	// Check total usage
	usage, err := c.GetInstanceWrites(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 25, int(usage.Units))
	assert.Equal(t, 3456, int(usage.SubUnits))
}

func setup(t *testing.T) *client.Client {
	apiPort, err := freeport.GetFreePort()
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := service.NewService(ctx, service.Config{
		ListenAddr:   util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", apiPort)),
		StripeAPIURL: "https://api.stripe.com",
		StripeKey:    "sk_test_RuU6Lq65WP23ykDSI9N9nRbC",
		DBURI:        "mongodb://127.0.0.1:27017/?replicaSet=rs0",
		DBName:       util.MakeToken(8),
		Debug:        true,
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
