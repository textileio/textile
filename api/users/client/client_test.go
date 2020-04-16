package client_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/apitest"
	"github.com/textileio/textile/api/common"
	c "github.com/textileio/textile/api/users/client"
	"github.com/textileio/textile/core"
	"google.golang.org/grpc"
)

func TestClient_ListThreads(t *testing.T) {
	t.Parallel()
	conf, client, threadsclient, done := setup(t)
	defer done()
	ctx := context.Background()

	t.Run("without session", func(t *testing.T) {
		_, err := client.ListThreads(ctx)
		require.NotNil(t, err)
	})

	user := apitest.Login(t, client, conf, apitest.NewEmail())

	t.Run("with session", func(t *testing.T) {
		ctx = common.NewSessionContext(ctx, user.Session)
		list, err := client.ListThreads(ctx)
		require.Nil(t, err)
		require.Empty(t, list.List)

		err = threadsclient.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.Nil(t, err)
		err = threadsclient.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.Nil(t, err)

		list, err = client.ListThreads(ctx)
		require.Nil(t, err)
		require.Equal(t, len(list.List), 2)
	})
}

func setup(t *testing.T) (core.Config, *c.Client, *tc.Client, func()) {
	conf, shutdown := apitest.MakeTextile(t)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrApi)
	require.Nil(t, err)
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{})}
	client, err := c.NewClient(target, opts...)
	require.Nil(t, err)
	threadsclient, err := tc.NewClient(target, opts...)
	require.Nil(t, err)

	return conf, client, threadsclient, func() {
		shutdown()
		err := client.Close()
		require.Nil(t, err)
		err = threadsclient.Close()
		require.Nil(t, err)
	}
}
