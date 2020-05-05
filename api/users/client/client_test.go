package client_test

import (
	"context"
	"crypto/rand"
	"os"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	nc "github.com/textileio/go-threads/net/api/client"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/apitest"
	bc "github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	hc "github.com/textileio/textile/api/hub/client"
	hubpb "github.com/textileio/textile/api/hub/pb"
	c "github.com/textileio/textile/api/users/client"
	"github.com/textileio/textile/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestClient_GetThread(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _, done := setup(t)
	defer done()
	ctx := context.Background()

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("bad keys", func(t *testing.T) {
		// No key
		_, err := client.GetThread(ctx, "foo")
		require.NotNil(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// No key signature
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_ACCOUNT)
		require.Nil(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)
		_, err = client.GetThread(ctx, "foo")
		require.NotNil(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// Old key signature
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(-time.Minute), key.Secret)
		require.Nil(t, err)
		_, err = client.GetThread(ctx, "foo")
		require.NotNil(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("account keys", func(t *testing.T) {
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_ACCOUNT)
		require.Nil(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
		require.Nil(t, err)

		// Not found
		_, err = client.GetThread(ctx, "foo")
		require.NotNil(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))

		// All good
		ctx = common.NewThreadNameContext(ctx, "foo")
		err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.Nil(t, err)
		res, err := client.GetThread(ctx, "foo")
		require.Nil(t, err)
		assert.Equal(t, "foo", res.Name)
		assert.True(t, res.IsDB)
	})

	t.Run("users keys", func(t *testing.T) {
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_USER)
		require.Nil(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
		require.Nil(t, err)

		// No token
		_, err = client.GetThread(ctx, "foo")
		require.NotNil(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// Not found
		sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
		require.Nil(t, err)
		tok, err := threads.GetToken(ctx, thread.NewLibp2pIdentity(sk))
		require.Nil(t, err)
		ctx = thread.NewTokenContext(ctx, tok)
		_, err = client.GetThread(ctx, "foo")
		require.NotNil(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))

		// All good
		ctx = common.NewThreadNameContext(ctx, "foo")
		err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.Nil(t, err)
		res, err := client.GetThread(ctx, "foo")
		require.Nil(t, err)
		assert.Equal(t, "foo", res.Name)
		assert.True(t, res.IsDB)
	})
}

func TestClient_ListThreads(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, net, _, done := setup(t)
	defer done()
	ctx := context.Background()

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("bad keys", func(t *testing.T) {
		// No key
		_, err := client.ListThreads(ctx)
		require.NotNil(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// No key signature
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_ACCOUNT)
		require.Nil(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)
		_, err = client.ListThreads(ctx)
		require.NotNil(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// Old key signature
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(-time.Minute), key.Secret)
		require.Nil(t, err)
		_, err = client.ListThreads(ctx)
		require.NotNil(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("account keys", func(t *testing.T) {
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_ACCOUNT)
		require.Nil(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
		require.Nil(t, err)

		// Empty
		res, err := client.ListThreads(ctx)
		require.Nil(t, err)
		assert.Equal(t, 0, len(res.List))

		// Got one
		_, err = net.CreateThread(ctx, thread.NewIDV1(thread.Raw, 32))
		require.Nil(t, err)
		res, err = client.ListThreads(ctx)
		require.Nil(t, err)
		assert.Equal(t, 1, len(res.List))
		assert.False(t, res.List[0].IsDB)
	})

	t.Run("users keys", func(t *testing.T) {
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_USER)
		require.Nil(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
		require.Nil(t, err)

		// No token
		_, err = client.ListThreads(ctx)
		require.NotNil(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// Empty
		sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
		require.Nil(t, err)
		tok, err := threads.GetToken(ctx, thread.NewLibp2pIdentity(sk))
		require.Nil(t, err)
		ctx = thread.NewTokenContext(ctx, tok)
		res, err := client.ListThreads(ctx)
		require.Nil(t, err)
		assert.Equal(t, 0, len(res.List))

		// Got one
		ctx = common.NewThreadNameContext(ctx, "foo")
		err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.Nil(t, err)
		res, err = client.ListThreads(ctx)
		require.Nil(t, err)
		assert.Equal(t, 1, len(res.List))
		assert.Equal(t, "foo", res.List[0].Name)
		assert.True(t, res.List[0].IsDB)
	})
}

func TestAccountBuckets(t *testing.T) {
	t.Parallel()
	conf, users, hub, threads, _, buckets, done := setup(t)
	defer done()
	ctx := context.Background()

	// Signup, create an API key, and sign it for the requests
	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	devCtx := common.NewSessionContext(ctx, dev.Session)
	key, err := hub.CreateKey(devCtx, hubpb.KeyType_ACCOUNT)
	require.Nil(t, err)
	ctx = common.NewAPIKeyContext(ctx, key.Key)
	ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
	require.Nil(t, err)

	// Create a db for the bucket
	ctx = common.NewThreadNameContext(ctx, "my-buckets")
	dbID := thread.NewIDV1(thread.Raw, 32)
	err = threads.NewDB(ctx, dbID)
	require.Nil(t, err)

	// Initialize a new bucket in the db
	ctx = common.NewThreadIDContext(ctx, dbID)
	buck, err := buckets.Init(ctx, "mybuck")
	require.Nil(t, err)

	// Finally, push a file to the bucket.
	file, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file.Close()
	_, file1Root, err := buckets.PushPath(ctx, buck.Root.Key, "file1.jpg", file)
	require.Nil(t, err)
	assert.NotEmpty(t, file1Root.String())

	// We should have a thread named "my-buckets"
	res, err := users.GetThread(ctx, "my-buckets")
	require.Nil(t, err)
	assert.Equal(t, dbID.Bytes(), res.ID)
}

func TestUserBuckets(t *testing.T) {
	t.Parallel()
	conf, users, hub, threads, _, buckets, done := setup(t)
	defer done()
	ctx := context.Background()

	// Signup, create an API key, and sign it for the requests
	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	devCtx := common.NewSessionContext(ctx, dev.Session)
	key, err := hub.CreateKey(devCtx, hubpb.KeyType_USER)
	require.Nil(t, err)
	ctx = common.NewAPIKeyContext(ctx, key.Key)
	ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
	require.Nil(t, err)

	// Generate a user identity and get a token for it
	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.Nil(t, err)
	tok, err := threads.GetToken(ctx, thread.NewLibp2pIdentity(sk))
	require.Nil(t, err)
	ctx = thread.NewTokenContext(ctx, tok)

	// Create a db for the bucket
	ctx = common.NewThreadNameContext(ctx, "my-buckets")
	dbID := thread.NewIDV1(thread.Raw, 32)
	err = threads.NewDB(ctx, dbID)
	require.Nil(t, err)

	// Initialize a new bucket in the db
	ctx = common.NewThreadIDContext(ctx, dbID)
	buck, err := buckets.Init(ctx, "mybuck")
	require.Nil(t, err)

	// Finally, push a file to the bucket.
	file, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file.Close()
	_, file1Root, err := buckets.PushPath(ctx, buck.Root.Key, "file1.jpg", file)
	require.Nil(t, err)
	assert.NotEmpty(t, file1Root.String())

	// We should have a thread named "my-buckets"
	res, err := users.GetThread(ctx, "my-buckets")
	require.Nil(t, err)
	assert.Equal(t, dbID.Bytes(), res.ID)

	// The dev should see that the key was used to create one thread
	keys, err := hub.ListKeys(devCtx)
	require.Nil(t, err)
	assert.Equal(t, 1, len(keys.List))
	assert.Equal(t, 1, int(keys.List[0].Threads))
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
