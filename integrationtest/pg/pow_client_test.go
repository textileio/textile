package pg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	tutil "github.com/textileio/go-threads/util"
	ffsRpc "github.com/textileio/powergate/ffs/rpc"
	"github.com/textileio/textile/v2/api/apitest"
	"github.com/textileio/textile/v2/api/common"
	hc "github.com/textileio/textile/v2/api/hub/client"
	pc "github.com/textileio/textile/v2/api/pow/client"
	"github.com/textileio/textile/v2/core"
	"github.com/textileio/textile/v2/util"
	"google.golang.org/grpc"
)

func TestPowClient(t *testing.T) {
	_ = StartPowergate(t)
	ctx, _, client, cleanup := setupPowClient(t)

	t.Run("Health", func(t *testing.T) {
		res, err := client.Health(ctx)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("Peers", func(t *testing.T) {
		res, err := client.Peers(ctx)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("FindPeer", func(t *testing.T) {
		res0, err := client.Peers(ctx)
		require.NoError(t, err)
		require.NotNil(t, res0)
		require.Len(t, res0.Peers, 1)

		res1, err := client.FindPeer(ctx, res0.Peers[0].AddrInfo.Id)
		require.NoError(t, err)
		require.NotNil(t, res1)
	})

	t.Run("Connectedness", func(t *testing.T) {
		res0, err := client.Peers(ctx)
		require.NoError(t, err)
		require.NotNil(t, res0)
		require.Len(t, res0.Peers, 1)

		res1, err := client.Connectedness(ctx, res0.Peers[0].AddrInfo.Id)
		require.NoError(t, err)
		require.NotNil(t, res1)
	})

	t.Run("Addrs", func(t *testing.T) {
		res, err := client.Addrs(ctx)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("NewAddr", func(t *testing.T) {
		res, err := client.NewAddr(ctx, "new one", "bls", false)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("Info", func(t *testing.T) {
		res, err := client.Info(ctx)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	// ToDo: Test Show, but need to add data to ffs to do that

	t.Run("ShowAll", func(t *testing.T) {
		res, err := client.ShowAll(ctx)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("ListStorageDealRecords", func(t *testing.T) {
		res, err := client.ListStorageDealRecords(ctx, &ffsRpc.ListDealRecordsConfig{IncludeFinal: true})
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("ListRetrievalDealRecords", func(t *testing.T) {
		res, err := client.ListRetrievalDealRecords(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("Balance", func(t *testing.T) {
		res0, err := client.Addrs(ctx)
		require.NoError(t, err)
		require.NotNil(t, res0)
		require.GreaterOrEqual(t, len(res0.Addrs), 1)

		res1, err := client.Balance(ctx, res0.Addrs[0].Addr)
		require.NoError(t, err)
		require.NotNil(t, res1)
	})

	cleanup(true)
}

func setupPowClient(t util.TestingTWithCleanup) (context.Context, core.Config, *pc.Client, func(bool)) {
	conf := apitest.DefaultTextileConfig(t)
	conf.AddrPowergateAPI = powAddr
	conf.AddrIPFSAPI = util.MustParseAddr("/ip4/127.0.0.1/tcp/5011")
	conf.AddrMongoURI = "mongodb://127.0.0.1:27027"
	shutdown := apitest.MakeTextileWithConfig(t, conf, false)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.NoError(t, err)
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{})}
	client, err := pc.NewClient(target, opts...)
	require.NoError(t, err)
	hubclient, err := hc.NewClient(target, opts...)
	require.NoError(t, err)

	user := apitest.Signup(t, hubclient, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	t.Cleanup(func() {
		err := client.Close()
		require.NoError(t, err)
	})

	return ctx, conf, client, shutdown
}
