package client_test

import (
	"context"
	"crypto/rand"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/require"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	nc "github.com/textileio/go-threads/net/api/client"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/apitest"
	bc "github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	hc "github.com/textileio/textile/api/hub/client"
	c "github.com/textileio/textile/api/users/client"
	"github.com/textileio/textile/core"
	"google.golang.org/grpc"
)

func TestClient_GetThread(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _, done := setup(t)
	defer done()
	ctx := context.Background()

	t.Run("without key", func(t *testing.T) {
		_, err := client.GetThread(ctx, "foo")
		require.NotNil(t, err)
	})

	dev := apitest.Login(t, hub, conf, apitest.NewEmail())
	key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session))
	require.Nil(t, err)
	ctx = common.NewAPIKeyContext(ctx, key.Key)

	t.Run("without key signature", func(t *testing.T) {
		_, err := client.GetThread(ctx, "foo")
		require.NotNil(t, err)
	})

	ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(-time.Minute), key.Secret)
	require.Nil(t, err)

	t.Run("with old key signature", func(t *testing.T) {
		_, err := client.GetThread(ctx, "foo")
		require.NotNil(t, err)
	})

	ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
	require.Nil(t, err)

	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	tok, err := threads.GetToken(ctx, thread.NewLibp2pIdentity(sk))
	require.Nil(t, err)
	ctx = thread.NewTokenContext(ctx, tok)
	err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
	require.Nil(t, err)

	t.Run("with key", func(t *testing.T) {
		_, err := client.GetThread(ctx, "foo")
		require.NotNil(t, err)

		ctx = common.NewThreadNameContext(ctx, "foo")
		err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.Nil(t, err)

		res, err := client.GetThread(ctx, "foo")
		require.Nil(t, err)
		require.Equal(t, "foo", res.Name)
	})
}

func TestClient_ListThreads(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, net, _, done := setup(t)
	defer done()
	ctx := context.Background()

	t.Run("without key", func(t *testing.T) {
		_, err := client.ListThreads(ctx)
		require.NotNil(t, err)
	})

	dev := apitest.Login(t, hub, conf, apitest.NewEmail())
	key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session))
	require.Nil(t, err)
	ctx = common.NewAPIKeyContext(ctx, key.Key)

	t.Run("with key, without token", func(t *testing.T) {
		_, err := client.ListThreads(ctx)
		require.NotNil(t, err)
	})

	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	tok, err := threads.GetToken(ctx, thread.NewLibp2pIdentity(sk))
	require.Nil(t, err)
	ctx = thread.NewTokenContext(ctx, tok)

	t.Run("with key, with token, without user", func(t *testing.T) {
		_, err := client.ListThreads(ctx)
		require.NotNil(t, err)
	})

	t.Run("with key, with token, with user", func(t *testing.T) {
		err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.Nil(t, err)
		err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.Nil(t, err)
		_, err = net.CreateThread(ctx, thread.NewIDV1(thread.Raw, 32))
		require.Nil(t, err)

		list, err := client.ListThreads(ctx)
		require.Nil(t, err)
		require.Equal(t, 3, len(list.List))
	})
}

func TestBuckets(t *testing.T) {
	t.Parallel()
	conf, _, hub, threads, _, buckets, done := setup(t)
	defer done()
	ctx := context.Background()

	dev := apitest.Login(t, hub, conf, apitest.NewEmail())
	key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session))
	require.Nil(t, err)
	ctx = common.NewAPIKeyContext(ctx, key.Key)

	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	tok, err := threads.GetToken(ctx, thread.NewLibp2pIdentity(sk))
	require.Nil(t, err)
	ctx = thread.NewTokenContext(ctx, tok)

	ctx = common.NewThreadNameContext(ctx, "my bucket")
	dbID := thread.NewIDV1(thread.Raw, 32)
	err = threads.NewDB(ctx, dbID)
	require.Nil(t, err)

	file, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file.Close()
	ctx = common.NewThreadIDContext(ctx, dbID)
	_, file1Root, err := buckets.PushPath(ctx, "mybuck1/file1.jpg", file)
	require.Nil(t, err)
	require.NotEmpty(t, file1Root.String())
}

func setup(t *testing.T) (core.Config, *c.Client, *hc.Client, *tc.Client, *nc.Client, *bc.Client, func()) {
	conf, shutdown := apitest.MakeTextile(t)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrApi)
	require.Nil(t, err)
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{})}
	client, err := c.NewClient(target, opts...)
	require.Nil(t, err)
	hubclient, err := hc.NewClient(target, opts...)
	require.Nil(t, err)
	threadsclient, err := tc.NewClient(target, opts...)
	require.Nil(t, err)
	threadsnetclient, err := nc.NewClient(target, opts...)
	require.Nil(t, err)
	bucketsclient, err := bc.NewClient(target, opts...)
	require.Nil(t, err)

	return conf, client, hubclient, threadsclient, threadsnetclient, bucketsclient, func() {
		shutdown()
		err := client.Close()
		require.Nil(t, err)
		err = threadsclient.Close()
		require.Nil(t, err)
	}
}
