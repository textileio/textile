package pg

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	tutil "github.com/textileio/go-threads/util"
	userPb "github.com/textileio/powergate/v2/api/gen/powergate/user/v1"
	"github.com/textileio/textile/v2/api/apitest"
	"github.com/textileio/textile/v2/api/common"
	fc "github.com/textileio/textile/v2/api/filecoin/client"
	hc "github.com/textileio/textile/v2/api/hubd/client"
	"github.com/textileio/textile/v2/core"
	"github.com/textileio/textile/v2/util"
	"google.golang.org/grpc"
)

func TestPowClient(t *testing.T) {
	_, _ = StartPowergate(t, true)
	ctx, _, client := setupPowClient(t)

	t.Run("Addresses", func(t *testing.T) {
		res, err := client.Addresses(ctx)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("Balance", func(t *testing.T) {
		res0, err := client.Addresses(ctx)
		require.NoError(t, err)
		require.NotNil(t, res0)
		require.GreaterOrEqual(t, len(res0.Addresses), 1)

		res1, err := client.Balance(ctx, res0.Addresses[0].Address)
		require.NoError(t, err)
		require.NotNil(t, res1)
	})

	t.Run("StorageDealRecords", func(t *testing.T) {
		res, err := client.StorageDealRecords(ctx, &userPb.DealRecordsConfig{IncludeFinal: true})
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("RetrievalDealRecords", func(t *testing.T) {
		res, err := client.RetrievalDealRecords(ctx, nil)
		require.NoError(t, err)
		require.NotNil(t, res)
	})
}

func setupPowClient(t util.TestingTWithCleanup) (context.Context, core.Config, *fc.Client) {
	conf := apitest.DefaultTextileConfig(t)
	conf.AddrPowergateAPI = powAddr
	apitest.MakeTextileWithConfig(t, conf)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.NoError(t, err)
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{})}
	client, err := fc.NewClient(target, opts...)
	require.NoError(t, err)
	hubclient, err := hc.NewClient(target, opts...)
	require.NoError(t, err)

	user := apitest.Signup(t, hubclient, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	t.Cleanup(func() {
		err := client.Close()
		require.NoError(t, err)
	})

	return ctx, conf, client
}
