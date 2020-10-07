package client_test

import (
	"context"
	"fmt"
	"testing"
	"time"

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
	units, changed, err := c.SetStoredData(context.Background(), id, 1*mib)
	require.NoError(t, err)
	assert.Equal(t, 0, int(units))
	assert.False(t, changed)

	// Units should round up
	units, changed, err = c.SetStoredData(context.Background(), id, 30*mib)
	require.NoError(t, err)
	assert.Equal(t, 1, int(units))
	assert.True(t, changed)

	// Units should round down
	units, changed, err = c.SetStoredData(context.Background(), id, 260*mib)
	require.NoError(t, err)
	assert.Equal(t, 5, int(units))
	assert.True(t, changed)
}

func TestClient_IncNetworkEgress(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Add some egress under unit size
	units, changed, err := c.IncNetworkEgress(context.Background(), id, 1*mib)
	require.NoError(t, err)
	assert.Equal(t, 0, int(units))
	assert.False(t, changed)

	// Add some egress to reach unit size
	units, changed, err = c.IncNetworkEgress(context.Background(), id, 99*mib)
	require.NoError(t, err)
	assert.Equal(t, 1, int(units))
	assert.True(t, changed)
	time.Sleep(time.Minute)

	// Add a bunch of egress
	units, changed, err = c.IncNetworkEgress(context.Background(), id, 1234*mib)
	require.NoError(t, err)
	assert.Equal(t, 12, int(units))
	assert.True(t, changed)
	time.Sleep(time.Minute)

	// Check remainder by adding enough egress to reach one more unit
	units, changed, err = c.IncNetworkEgress(context.Background(), id, 66*mib)
	require.NoError(t, err)
	assert.Equal(t, 13, int(units))
	assert.True(t, changed)
}

func TestClient_IncInstanceReads(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Add some reads under unit size
	units, changed, err := c.IncInstanceReads(context.Background(), id, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, int(units))
	assert.False(t, changed)

	// Add some reads to reach unit size
	units, changed, err = c.IncInstanceReads(context.Background(), id, 9999)
	require.NoError(t, err)
	assert.Equal(t, 1, int(units))
	assert.True(t, changed)

	// Add a bunch of reads
	units, changed, err = c.IncInstanceReads(context.Background(), id, 123456)
	require.NoError(t, err)
	assert.Equal(t, 12, int(units))
	assert.True(t, changed)

	// Check remainder by adding enough reads to reach one more unit
	units, changed, err = c.IncInstanceReads(context.Background(), id, 6544)
	require.NoError(t, err)
	assert.Equal(t, 13, int(units))
	assert.True(t, changed)
}

func TestClient_IncInstanceWrites(t *testing.T) {
	t.Parallel()
	c := setup(t)
	id, err := c.CreateCustomer(context.Background(), apitest.NewEmail())
	require.NoError(t, err)

	// Add some writes under unit size
	units, changed, err := c.IncInstanceWrites(context.Background(), id, 1)
	require.NoError(t, err)
	assert.Equal(t, 0, int(units))
	assert.False(t, changed)
	time.Sleep(time.Second)

	// Add some writes to reach unit size
	units, changed, err = c.IncInstanceWrites(context.Background(), id, 4999)
	require.NoError(t, err)
	assert.Equal(t, 1, int(units))
	assert.True(t, changed)
	time.Sleep(time.Second)

	// Add a bunch of writes
	units, changed, err = c.IncInstanceWrites(context.Background(), id, 123456)
	require.NoError(t, err)
	assert.Equal(t, 25, int(units))
	assert.True(t, changed)
	time.Sleep(time.Second)

	// Check remainder by adding enough writes to reach one more unit
	units, changed, err = c.IncInstanceWrites(context.Background(), id, 3456)
	require.NoError(t, err)
	assert.Equal(t, 26, int(units))
	assert.True(t, changed)
	time.Sleep(time.Second)
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
		ListenAddr: util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", apiPort)),
		//StripeAPIURL: "http://127.0.0.1:8420",
		//StripeKey:    "sk_test_123",
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
