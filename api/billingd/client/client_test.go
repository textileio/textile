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
	mib = 1048576
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

func TestClient_SetStoredData(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Units should round down
	res, err := c.SetStoredData(context.Background(), id, 1*mib)
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
	usage, err := c.GetPeriodUsage(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 5, int(usage.StoredData.Units))
	assert.Equal(t, 260*mib, int(usage.StoredData.TotalSize))
}

func TestClient_IncNetworkEgress(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Add some egress under unit size
	res, err := c.IncNetworkEgress(context.Background(), id, 1*mib)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.AddedUnits))
	assert.Equal(t, 1*mib, int(res.SubUnits))

	// Add some egress to reach unit size
	res, err = c.IncNetworkEgress(context.Background(), id, 99*mib)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.AddedUnits))
	assert.Equal(t, 0, int(res.SubUnits))

	// Add a bunch of egress
	res, err = c.IncNetworkEgress(context.Background(), id, 1234*mib)
	require.NoError(t, err)
	assert.Equal(t, 12, int(res.AddedUnits))
	assert.Equal(t, 34*mib, int(res.SubUnits))

	// Check remainder by adding enough egress to reach one more unit
	res, err = c.IncNetworkEgress(context.Background(), id, 66*mib)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.AddedUnits))
	assert.Equal(t, 0, int(res.SubUnits))

	// Check total usage
	usage, err := c.GetPeriodUsage(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 14, int(usage.NetworkEgress.Units))
	assert.Equal(t, 0, int(usage.NetworkEgress.SubUnits))
}

func TestClient_IncInstanceReads(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Add some reads under unit size
	res, err := c.IncInstanceReads(context.Background(), id, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.AddedUnits))
	assert.Equal(t, 1, int(res.SubUnits))

	// Add some reads to reach unit size
	res, err = c.IncInstanceReads(context.Background(), id, 9999)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.AddedUnits))
	assert.Equal(t, 0, int(res.SubUnits))

	// Add a bunch of reads
	res, err = c.IncInstanceReads(context.Background(), id, 123456)
	require.NoError(t, err)
	assert.Equal(t, 12, int(res.AddedUnits))
	assert.Equal(t, 3456, int(res.SubUnits))

	// Check remainder by adding enough reads to reach one more unit
	res, err = c.IncInstanceReads(context.Background(), id, 6544)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.AddedUnits))
	assert.Equal(t, 0, int(res.SubUnits))

	// Check total usage
	usage, err := c.GetPeriodUsage(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 14, int(usage.InstanceReads.Units))
	assert.Equal(t, 0, int(usage.InstanceReads.SubUnits))
}

func TestClient_IncInstanceWrites(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Add some writes under unit size
	res, err := c.IncInstanceWrites(context.Background(), id, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, int(res.AddedUnits))
	assert.Equal(t, 1, int(res.SubUnits))

	// Add some writes to reach unit size
	res, err = c.IncInstanceWrites(context.Background(), id, 4999)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.AddedUnits))
	assert.Equal(t, 0, int(res.SubUnits))

	// Add a bunch of writes
	res, err = c.IncInstanceWrites(context.Background(), id, 123456)
	require.NoError(t, err)
	assert.Equal(t, 24, int(res.AddedUnits))
	assert.Equal(t, 3456, int(res.SubUnits))

	// Check remainder by adding enough writes to reach one more unit
	res, err = c.IncInstanceWrites(context.Background(), id, 1544)
	require.NoError(t, err)
	assert.Equal(t, 1, int(res.AddedUnits))
	assert.Equal(t, 0, int(res.SubUnits))

	// Check total usage
	usage, err := c.GetPeriodUsage(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, 26, int(usage.InstanceWrites.Units))
	assert.Equal(t, 0, int(usage.InstanceWrites.SubUnits))
}

func TestClient_DeleteCustomer(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	err = c.DeleteCustomer(context.Background(), id)
	require.NoError(t, err)
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
