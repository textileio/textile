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
	conf, client, hub, threads, _, _ := setup(t)
	ctx := context.Background()

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("bad keys", func(t *testing.T) {
		// No key
		_, err := client.GetThread(ctx, "foo")
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// No key signature
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_ACCOUNT, true)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)
		_, err = client.GetThread(ctx, "foo")
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// Old key signature
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(-time.Minute), key.Secret)
		require.NoError(t, err)
		_, err = client.GetThread(ctx, "foo")
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("account keys", func(t *testing.T) {
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_ACCOUNT, true)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
		require.NoError(t, err)

		// Not found
		_, err = client.GetThread(ctx, "foo")
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))

		// All good
		ctx = common.NewThreadNameContext(ctx, "foo")
		err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.NoError(t, err)
		res, err := client.GetThread(ctx, "foo")
		require.NoError(t, err)
		assert.Equal(t, "foo", res.Name)
		assert.True(t, res.IsDB)
	})

	t.Run("users keys", func(t *testing.T) {
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_USER, true)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
		require.NoError(t, err)

		// No token
		_, err = client.GetThread(ctx, "foo")
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// Not found
		sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
		require.NoError(t, err)
		tok, err := threads.GetToken(ctx, thread.NewLibp2pIdentity(sk))
		require.NoError(t, err)
		ctx = thread.NewTokenContext(ctx, tok)
		_, err = client.GetThread(ctx, "foo")
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))

		// All good
		ctx = common.NewThreadNameContext(ctx, "foo")
		err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.NoError(t, err)
		res, err := client.GetThread(ctx, "foo")
		require.NoError(t, err)
		assert.Equal(t, "foo", res.Name)
		assert.True(t, res.IsDB)
	})

	t.Run("insecure keys", func(t *testing.T) {
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_ACCOUNT, false)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)

		// All good
		ctx = common.NewThreadNameContext(ctx, "foo2")
		err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.NoError(t, err)
		res, err := client.GetThread(ctx, "foo2")
		require.NoError(t, err)
		assert.Equal(t, "foo2", res.Name)
		assert.True(t, res.IsDB)
	})
}

func TestClient_CreateThreadsLimit(t *testing.T) {
	t.Parallel()
	conf := apitest.DefaultTextileConfig(t)
	conf.ThreadsMaxNumberPerOwner = 1
	conf, _, hub, _, net, _ := setupWithConf(t, conf)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())

	ctx := context.Background()
	key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_ACCOUNT, true)
	require.NoError(t, err)
	ctx = common.NewAPIKeyContext(ctx, key.Key)
	ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
	require.NoError(t, err)

	// First thread allowed.
	_, err = net.CreateThread(ctx, thread.NewIDV1(thread.Raw, 32))
	require.NoError(t, err)

	// Second one should exceed limit.
	_, err = net.CreateThread(ctx, thread.NewIDV1(thread.Raw, 32))
	require.Error(t, err)
	require.Contains(t, err.Error(), core.ErrTooManyThreadsPerOwner.Error())
}

func TestClient_ListThreads(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, net, _ := setup(t)
	ctx := context.Background()

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("bad keys", func(t *testing.T) {
		// No key
		_, err := client.ListThreads(ctx)
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// No key signature
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_ACCOUNT, true)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)
		_, err = client.ListThreads(ctx)
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// Old key signature
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(-time.Minute), key.Secret)
		require.NoError(t, err)
		_, err = client.ListThreads(ctx)
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("account keys", func(t *testing.T) {
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_ACCOUNT, true)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
		require.NoError(t, err)

		// Empty
		res, err := client.ListThreads(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, len(res.List))

		// Got one
		_, err = net.CreateThread(ctx, thread.NewIDV1(thread.Raw, 32))
		require.NoError(t, err)
		res, err = client.ListThreads(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, len(res.List))
		assert.False(t, res.List[0].IsDB)
	})

	t.Run("users keys", func(t *testing.T) {
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_USER, true)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
		require.NoError(t, err)

		// No token
		_, err = client.ListThreads(ctx)
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// Empty
		sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
		require.NoError(t, err)
		tok, err := threads.GetToken(ctx, thread.NewLibp2pIdentity(sk))
		require.NoError(t, err)
		ctx = thread.NewTokenContext(ctx, tok)
		res, err := client.ListThreads(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, len(res.List))

		// Got one
		ctx = common.NewThreadNameContext(ctx, "foo")
		err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.NoError(t, err)
		res, err = client.ListThreads(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, len(res.List))
		assert.Equal(t, "foo", res.List[0].Name)
		assert.True(t, res.List[0].IsDB)
	})

	t.Run("insecure keys", func(t *testing.T) {
		key, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_ACCOUNT, false)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, key.Key)

		// Got one
		res, err := client.ListThreads(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, len(res.List))

		// Got two
		_, err = net.CreateThread(ctx, thread.NewIDV1(thread.Raw, 32))
		require.NoError(t, err)
		res, err = client.ListThreads(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, len(res.List))
		assert.False(t, res.List[1].IsDB)
	})
}

func TestClient_SetupMail(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	key, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_USER, false)
	require.NoError(t, err)

	ctx := common.NewAPIKeyContext(context.Background(), key.Key)
	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	from := thread.NewLibp2pIdentity(sk)
	tok, err := threads.GetToken(ctx, from)
	require.NoError(t, err)
	ctx = thread.NewTokenContext(ctx, tok)

	inbox, sentbox, err := client.SetupMail(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, inbox)
	assert.NotEmpty(t, sentbox)

	// should be idempotent
	_, _, err = client.SetupMail(ctx)
	require.NoError(t, err)
}

func TestClient_SendMessage(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	key, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_USER, false)
	require.NoError(t, err)

	from, fctx := setupUserMail(t, client, threads, key.Key)
	to, _ := setupUserMail(t, client, threads, key.Key)

	res, err := client.SendMessage(fctx, from, to.GetPublic(), []byte("howdy"))
	require.NoError(t, err)
	assert.NotEmpty(t, res.ID)
	assert.NotEmpty(t, res.CreatedAt)
}

func TestClient_ListInboxMessages(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	key, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_USER, false)
	require.NoError(t, err)

	from, fctx := setupUserMail(t, client, threads, key.Key)
	to, tctx := setupUserMail(t, client, threads, key.Key)

	var i int
	var readID string
	for i < 100 {
		res, err := client.SendMessage(fctx, from, to.GetPublic(), []byte("hi"))
		require.NoError(t, err)
		if i == 0 {
			readID = res.ID
		}
		i++
	}

	t.Run("check order", func(t *testing.T) {
		list, err := client.ListInboxMessages(tctx, to)
		require.NoError(t, err)
		assert.Len(t, list, 100)
		for i := 0; i < len(list)-1; i++ {
			assert.True(t, list[i].CreatedAt.After(list[i+1].CreatedAt))
		}
		assert.Equal(t, "hi", string(list[0].Body))
	})

	t.Run("with limit", func(t *testing.T) {
		list, err := client.ListInboxMessages(tctx, to, c.WithLimit(10))
		require.NoError(t, err)
		assert.Len(t, list, 10)
	})

	t.Run("with ascending", func(t *testing.T) {
		list, err := client.ListInboxMessages(tctx, to, c.WithLimit(10), c.WithAscending(true))
		require.NoError(t, err)
		assert.Len(t, list, 10)
		for i := 0; i < len(list)-1; i++ {
			assert.True(t, list[i].CreatedAt.Before(list[i+1].CreatedAt))
		}
	})

	t.Run("with seek", func(t *testing.T) {
		var all []c.Message
		var seek string
		pageSize := 10
		for { // 10 pages
			list, err := client.ListInboxMessages(tctx, to, c.WithLimit(pageSize+1), c.WithSeek(seek))
			require.NoError(t, err)
			all = append(all, list[:pageSize]...)
			if len(list) < pageSize+1 {
				break
			}
			seek = list[len(list)-1].ID
		}
		assert.Len(t, all, 100)
		for i := 0; i < len(all)-1; i++ {
			assert.True(t, all[i].CreatedAt.After(all[i+1].CreatedAt))
		}
	})

	t.Run("with status", func(t *testing.T) {
		err = client.ReadInboxMessage(tctx, readID)
		require.NoError(t, err)

		list1, err := client.ListInboxMessages(tctx, to, c.WithStatus(c.All))
		require.NoError(t, err)
		assert.Len(t, list1, 100)
		list2, err := client.ListInboxMessages(tctx, to, c.WithStatus(c.Read))
		require.NoError(t, err)
		assert.Len(t, list2, 1)
		assert.False(t, list2[0].ReadAt.IsZero())
		list3, err := client.ListInboxMessages(tctx, to, c.WithStatus(c.Unread))
		require.NoError(t, err)
		assert.Len(t, list3, 99)
		assert.True(t, list3[0].ReadAt.IsZero())
	})
}

func TestClient_ListSentMessages(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	key, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_USER, false)
	require.NoError(t, err)

	from, fctx := setupUserMail(t, client, threads, key.Key)
	to, _ := setupUserMail(t, client, threads, key.Key)

	_, err = client.SendMessage(fctx, from, to.GetPublic(), []byte("one"))
	require.NoError(t, err)
	_, err = client.SendMessage(fctx, from, to.GetPublic(), []byte("two"))
	require.NoError(t, err)

	list, err := client.ListSentMessages(fctx, from)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, "two", string(list[0].Body))
	assert.Equal(t, "one", string(list[1].Body))
}

func TestClient_ReadInboxMessage(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	key, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_USER, false)
	require.NoError(t, err)

	from, fctx := setupUserMail(t, client, threads, key.Key)
	to, tctx := setupUserMail(t, client, threads, key.Key)

	res, err := client.SendMessage(fctx, from, to.GetPublic(), []byte("howdy"))
	require.NoError(t, err)

	err = client.ReadInboxMessage(tctx, res.ID)
	require.NoError(t, err)

	list, err := client.ListInboxMessages(tctx, to)
	require.NoError(t, err)
	assert.False(t, list[0].ReadAt.IsZero())
}

func TestClient_DeleteInboxMessage(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	key, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_USER, false)
	require.NoError(t, err)

	from, fctx := setupUserMail(t, client, threads, key.Key)
	to, tctx := setupUserMail(t, client, threads, key.Key)

	res, err := client.SendMessage(fctx, from, to.GetPublic(), []byte("howdy"))
	require.NoError(t, err)

	err = client.DeleteInboxMessage(tctx, res.ID)
	require.NoError(t, err)

	list, err := client.ListInboxMessages(tctx, to)
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

func TestClient_DeleteSentMessage(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	key, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_USER, false)
	require.NoError(t, err)

	from, fctx := setupUserMail(t, client, threads, key.Key)
	to, _ := setupUserMail(t, client, threads, key.Key)

	res, err := client.SendMessage(fctx, from, to.GetPublic(), []byte("howdy"))
	require.NoError(t, err)

	err = client.DeleteSentMessage(fctx, res.ID)
	require.NoError(t, err)

	list, err := client.ListSentMessages(fctx, to)
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

func setupUserMail(t *testing.T, client *c.Client, threads *tc.Client, key string) (thread.Identity, context.Context) {
	ctx := common.NewAPIKeyContext(context.Background(), key)
	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	id := thread.NewLibp2pIdentity(sk)
	tok, err := threads.GetToken(ctx, id)
	require.NoError(t, err)
	ctx = thread.NewTokenContext(ctx, tok)
	_, _, err = client.SetupMail(ctx)
	require.NoError(t, err)
	return id, ctx
}

func TestAccountBuckets(t *testing.T) {
	t.Parallel()
	conf, users, hub, threads, _, buckets := setup(t)
	ctx := context.Background()

	// Signup, create an API key, and sign it for the requests
	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	devCtx := common.NewSessionContext(ctx, dev.Session)
	key, err := hub.CreateKey(devCtx, hubpb.KeyType_ACCOUNT, true)
	require.NoError(t, err)
	ctx = common.NewAPIKeyContext(ctx, key.Key)
	ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
	require.NoError(t, err)

	// Create a db for the bucket
	ctx = common.NewThreadNameContext(ctx, "my-buckets")
	dbID := thread.NewIDV1(thread.Raw, 32)
	err = threads.NewDB(ctx, dbID)
	require.NoError(t, err)

	// Initialize a new bucket in the db
	ctx = common.NewThreadIDContext(ctx, dbID)
	buck, err := buckets.Init(ctx)
	require.NoError(t, err)

	// Finally, push a file to the bucket.
	file, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file.Close()
	_, file1Root, err := buckets.PushPath(ctx, buck.Root.Key, "file1.jpg", file)
	require.NoError(t, err)
	assert.NotEmpty(t, file1Root.String())

	// We should have a thread named "my-buckets"
	res, err := users.GetThread(ctx, "my-buckets")
	require.NoError(t, err)
	assert.Equal(t, dbID.Bytes(), res.ID)
}

func TestUserBuckets(t *testing.T) {
	t.Parallel()
	conf, users, hub, threads, _, buckets := setup(t)
	ctx := context.Background()

	// Signup, create an API key, and sign it for the requests
	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	devCtx := common.NewSessionContext(ctx, dev.Session)
	key, err := hub.CreateKey(devCtx, hubpb.KeyType_USER, true)
	require.NoError(t, err)
	ctx = common.NewAPIKeyContext(ctx, key.Key)
	ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), key.Secret)
	require.NoError(t, err)

	// Generate a user identity and get a token for it
	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	tok, err := threads.GetToken(ctx, thread.NewLibp2pIdentity(sk))
	require.NoError(t, err)
	ctx = thread.NewTokenContext(ctx, tok)

	// Create a db for the bucket
	ctx = common.NewThreadNameContext(ctx, "my-buckets")
	dbID := thread.NewIDV1(thread.Raw, 32)
	err = threads.NewDB(ctx, dbID)
	require.NoError(t, err)

	// Initialize a new bucket in the db
	ctx = common.NewThreadIDContext(ctx, dbID)
	buck, err := buckets.Init(ctx)
	require.NoError(t, err)

	// Finally, push a file to the bucket.
	file, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file.Close()
	_, file1Root, err := buckets.PushPath(ctx, buck.Root.Key, "file1.jpg", file)
	require.NoError(t, err)
	assert.NotEmpty(t, file1Root.String())

	// We should have a thread named "my-buckets"
	res, err := users.GetThread(ctx, "my-buckets")
	require.NoError(t, err)
	assert.Equal(t, dbID.Bytes(), res.ID)

	// The dev should see that the key was used to create one thread
	keys, err := hub.ListKeys(devCtx)
	require.NoError(t, err)
	assert.Equal(t, 1, len(keys.List))
	assert.Equal(t, 1, int(keys.List[0].Threads))
}

func setup(t *testing.T) (core.Config, *c.Client, *hc.Client, *tc.Client, *nc.Client, *bc.Client) {
	defConfig := apitest.DefaultTextileConfig(t)
	return setupWithConf(t, defConfig)
}

func setupWithConf(t *testing.T, conf core.Config) (core.Config, *c.Client, *hc.Client, *tc.Client, *nc.Client, *bc.Client) {
	apitest.MakeTextileWithConfig(t, conf, true)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.NoError(t, err)
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{})}
	client, err := c.NewClient(target, opts...)
	require.NoError(t, err)
	hubclient, err := hc.NewClient(target, opts...)
	require.NoError(t, err)
	threadsclient, err := tc.NewClient(target, opts...)
	require.NoError(t, err)
	threadsnetclient, err := nc.NewClient(target, opts...)
	require.NoError(t, err)
	bucketsclient, err := bc.NewClient(target, opts...)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := client.Close()
		require.NoError(t, err)
		err = threadsclient.Close()
		require.NoError(t, err)
	})
	return conf, client, hubclient, threadsclient, threadsnetclient, bucketsclient
}
