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
	corenet "github.com/textileio/go-threads/core/net"
	"github.com/textileio/go-threads/core/thread"
	nc "github.com/textileio/go-threads/net/api/client"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/apitest"
	bc "github.com/textileio/textile/v2/api/bucketsd/client"
	"github.com/textileio/textile/v2/api/common"
	hc "github.com/textileio/textile/v2/api/hubd/client"
	hubpb "github.com/textileio/textile/v2/api/hubd/pb"
	c "github.com/textileio/textile/v2/api/usersd/client"
	bucks "github.com/textileio/textile/v2/buckets"
	"github.com/textileio/textile/v2/core"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMain(m *testing.M) {
	cleanup := func() {}
	if os.Getenv("SKIP_SERVICES") != "true" {
		cleanup = apitest.StartServices()
	}
	exitVal := m.Run()
	cleanup()
	os.Exit(exitVal)
}

func TestClient_GetThread(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t, nil)
	ctx := context.Background()

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("bad keys", func(t *testing.T) {
		// No key
		_, err := client.GetThread(ctx, "foo")
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// No key signature
		res, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_KEY_TYPE_ACCOUNT, true)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, res.KeyInfo.Key)
		_, err = client.GetThread(ctx, "foo")
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// Old key signature
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(-time.Minute), res.KeyInfo.Secret)
		require.NoError(t, err)
		_, err = client.GetThread(ctx, "foo")
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("account keys", func(t *testing.T) {
		res, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_KEY_TYPE_ACCOUNT, true)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, res.KeyInfo.Key)
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), res.KeyInfo.Secret)
		require.NoError(t, err)

		// Not found
		_, err = client.GetThread(ctx, "foo")
		require.Error(t, err)
		assert.Equal(t, codes.NotFound, status.Code(err))

		// All good
		ctx = common.NewThreadNameContext(ctx, "foo")
		err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.NoError(t, err)
		res2, err := client.GetThread(ctx, "foo")
		require.NoError(t, err)
		assert.Equal(t, "foo", res2.Name)
		assert.True(t, res2.IsDb)
	})

	t.Run("users keys", func(t *testing.T) {
		res, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_KEY_TYPE_USER, true)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, res.KeyInfo.Key)
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), res.KeyInfo.Secret)
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
		res2, err := client.GetThread(ctx, "foo")
		require.NoError(t, err)
		assert.Equal(t, "foo", res2.Name)
		assert.True(t, res2.IsDb)
	})

	t.Run("insecure keys", func(t *testing.T) {
		res, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_KEY_TYPE_ACCOUNT, false)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, res.KeyInfo.Key)

		// All good
		ctx = common.NewThreadNameContext(ctx, "foo2")
		err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.NoError(t, err)
		res2, err := client.GetThread(ctx, "foo2")
		require.NoError(t, err)
		assert.Equal(t, "foo2", res2.Name)
		assert.True(t, res2.IsDb)
	})
}

func TestClient_ListThreads(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, net, _ := setup(t, nil)
	ctx := context.Background()

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("bad keys", func(t *testing.T) {
		// No key
		_, err := client.ListThreads(ctx)
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// No key signature
		res, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_KEY_TYPE_ACCOUNT, true)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, res.KeyInfo.Key)
		_, err = client.ListThreads(ctx)
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))

		// Old key signature
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(-time.Minute), res.KeyInfo.Secret)
		require.NoError(t, err)
		_, err = client.ListThreads(ctx)
		require.Error(t, err)
		assert.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("account keys", func(t *testing.T) {
		res, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_KEY_TYPE_ACCOUNT, true)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, res.KeyInfo.Key)
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), res.KeyInfo.Secret)
		require.NoError(t, err)

		// Empty
		res2, err := client.ListThreads(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, len(res2.List))

		// Got one
		_, err = net.CreateThread(ctx, thread.NewIDV1(thread.Raw, 32))
		require.NoError(t, err)
		res3, err := client.ListThreads(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, len(res3.List))
		assert.False(t, res3.List[0].IsDb)
	})

	t.Run("users keys", func(t *testing.T) {
		res, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_KEY_TYPE_USER, true)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, res.KeyInfo.Key)
		ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), res.KeyInfo.Secret)
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
		res2, err := client.ListThreads(ctx)
		require.NoError(t, err)
		assert.Equal(t, 0, len(res2.List))

		// Got one
		ctx = common.NewThreadNameContext(ctx, "foo")
		err = threads.NewDB(ctx, thread.NewIDV1(thread.Raw, 32))
		require.NoError(t, err)
		res3, err := client.ListThreads(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, len(res3.List))
		assert.Equal(t, "foo", res3.List[0].Name)
		assert.True(t, res3.List[0].IsDb)
	})

	t.Run("insecure keys", func(t *testing.T) {
		res, err := hub.CreateKey(common.NewSessionContext(ctx, dev.Session), hubpb.KeyType_KEY_TYPE_ACCOUNT, false)
		require.NoError(t, err)
		ctx := common.NewAPIKeyContext(ctx, res.KeyInfo.Key)

		// Got one
		res2, err := client.ListThreads(ctx)
		require.NoError(t, err)
		assert.Equal(t, 1, len(res2.List))

		// Got two
		_, err = net.CreateThread(ctx, thread.NewIDV1(thread.Raw, 32))
		require.NoError(t, err)
		res3, err := client.ListThreads(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, len(res3.List))
		assert.False(t, res3.List[1].IsDb)
	})
}

func TestClient_SetupMailbox(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t, nil)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	res, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_KEY_TYPE_USER, false)
	require.NoError(t, err)

	ctx := common.NewAPIKeyContext(context.Background(), res.KeyInfo.Key)
	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	from := thread.NewLibp2pIdentity(sk)
	tok, err := threads.GetToken(ctx, from)
	require.NoError(t, err)
	ctx = thread.NewTokenContext(ctx, tok)

	mailbox, err := client.SetupMailbox(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, mailbox)

	// should be idempotent
	mailbox2, err := client.SetupMailbox(ctx)
	require.NoError(t, err)
	assert.Equal(t, mailbox, mailbox2)
}

func TestClient_SendMessage(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t, nil)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	res, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_KEY_TYPE_USER, false)
	require.NoError(t, err)

	from, fctx := setupUserMail(t, client, threads, res.KeyInfo.Key)
	to, _ := setupUserMail(t, client, threads, res.KeyInfo.Key)

	msg, err := client.SendMessage(fctx, from, to.GetPublic(), []byte("howdy"))
	require.NoError(t, err)
	assert.NotEmpty(t, msg.ID)
	assert.NotEmpty(t, msg.CreatedAt)
}

func TestClient_ListInboxMessages(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t, nil)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	res, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_KEY_TYPE_USER, false)
	require.NoError(t, err)

	from, fctx := setupUserMail(t, client, threads, res.KeyInfo.Key)
	to, tctx := setupUserMail(t, client, threads, res.KeyInfo.Key)

	var i int
	var readID string
	for i < 10 {
		res, err := client.SendMessage(fctx, from, to.GetPublic(), []byte("hi"))
		require.NoError(t, err)
		if i == 0 {
			readID = res.ID
		}
		i++
	}

	t.Run("check order", func(t *testing.T) {
		list, err := client.ListInboxMessages(tctx)
		require.NoError(t, err)
		assert.Len(t, list, 10)
		for i := 0; i < len(list)-1; i++ {
			assert.True(t, list[i].CreatedAt.After(list[i+1].CreatedAt))
		}
		m, err := list[0].Open(context.Background(), to)
		require.NoError(t, err)
		assert.Equal(t, "hi", string(m))
	})

	t.Run("with limit", func(t *testing.T) {
		list, err := client.ListInboxMessages(tctx, c.WithLimit(5))
		require.NoError(t, err)
		assert.Len(t, list, 5)
	})

	t.Run("with ascending", func(t *testing.T) {
		list, err := client.ListInboxMessages(tctx, c.WithLimit(5), c.WithAscending(true))
		require.NoError(t, err)
		assert.Len(t, list, 5)
		for i := 0; i < len(list)-1; i++ {
			assert.True(t, list[i].CreatedAt.Before(list[i+1].CreatedAt))
		}
	})

	t.Run("with seek", func(t *testing.T) {
		var all []c.Message
		var seek string
		pageSize := 2
		for { // 5 pages
			list, err := client.ListInboxMessages(tctx, c.WithLimit(pageSize+1), c.WithSeek(seek))
			require.NoError(t, err)
			all = append(all, list[:pageSize]...)
			if len(list) < pageSize+1 {
				break
			}
			seek = list[len(list)-1].ID
		}
		assert.Len(t, all, 10)
		for i := 0; i < len(all)-1; i++ {
			assert.True(t, all[i].CreatedAt.After(all[i+1].CreatedAt))
		}
	})

	t.Run("with status", func(t *testing.T) {
		err = client.ReadInboxMessage(tctx, readID)
		require.NoError(t, err)

		list1, err := client.ListInboxMessages(tctx, c.WithStatus(c.All))
		require.NoError(t, err)
		assert.Len(t, list1, 10)
		list2, err := client.ListInboxMessages(tctx, c.WithStatus(c.Read))
		require.NoError(t, err)
		assert.Len(t, list2, 1)
		assert.False(t, list2[0].ReadAt.IsZero())
		list3, err := client.ListInboxMessages(tctx, c.WithStatus(c.Unread))
		require.NoError(t, err)
		assert.Len(t, list3, 9)
		assert.True(t, list3[0].ReadAt.IsZero())
	})
}

func TestClient_ListSentboxMessages(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t, nil)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	res, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_KEY_TYPE_USER, false)
	require.NoError(t, err)

	from, fctx := setupUserMail(t, client, threads, res.KeyInfo.Key)
	to, _ := setupUserMail(t, client, threads, res.KeyInfo.Key)

	_, err = client.SendMessage(fctx, from, to.GetPublic(), []byte("one"))
	require.NoError(t, err)
	_, err = client.SendMessage(fctx, from, to.GetPublic(), []byte("two"))
	require.NoError(t, err)

	list, err := client.ListSentboxMessages(fctx)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	m2, err := list[0].Open(context.Background(), from)
	require.NoError(t, err)
	m1, err := list[1].Open(context.Background(), from)
	require.NoError(t, err)
	assert.Equal(t, "two", string(m2))
	assert.Equal(t, "one", string(m1))
}

func TestClient_ReadInboxMessage(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t, nil)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	res, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_KEY_TYPE_USER, false)
	require.NoError(t, err)

	from, fctx := setupUserMail(t, client, threads, res.KeyInfo.Key)
	to, tctx := setupUserMail(t, client, threads, res.KeyInfo.Key)

	msg, err := client.SendMessage(fctx, from, to.GetPublic(), []byte("howdy"))
	require.NoError(t, err)

	err = client.ReadInboxMessage(tctx, msg.ID)
	require.NoError(t, err)

	list, err := client.ListInboxMessages(tctx)
	require.NoError(t, err)
	assert.False(t, list[0].ReadAt.IsZero())
}

func TestClient_DeleteInboxMessage(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t, nil)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	res, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_KEY_TYPE_USER, false)
	require.NoError(t, err)

	from, fctx := setupUserMail(t, client, threads, res.KeyInfo.Key)
	to, tctx := setupUserMail(t, client, threads, res.KeyInfo.Key)

	msg, err := client.SendMessage(fctx, from, to.GetPublic(), []byte("howdy"))
	require.NoError(t, err)

	err = client.DeleteInboxMessage(tctx, msg.ID)
	require.NoError(t, err)

	list, err := client.ListInboxMessages(tctx)
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

func TestClient_DeleteSentboxMessage(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setup(t, nil)

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	res, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_KEY_TYPE_USER, false)
	require.NoError(t, err)

	from, fctx := setupUserMail(t, client, threads, res.KeyInfo.Key)
	to, _ := setupUserMail(t, client, threads, res.KeyInfo.Key)

	msg, err := client.SendMessage(fctx, from, to.GetPublic(), []byte("howdy"))
	require.NoError(t, err)

	err = client.DeleteSentboxMessage(fctx, msg.ID)
	require.NoError(t, err)

	list, err := client.ListSentboxMessages(fctx)
	require.NoError(t, err)
	assert.Len(t, list, 0)
}

func TestClient_GetUsage(t *testing.T) {
	t.Parallel()
	conf, client, hub, threads, _, _ := setupWithBilling(t)
	ctx := context.Background()

	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	res, err := hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_KEY_TYPE_USER, false)
	require.NoError(t, err)

	ctx = common.NewAPIKeyContext(context.Background(), res.KeyInfo.Key)
	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	id := thread.NewLibp2pIdentity(sk)
	tok, err := threads.GetToken(ctx, id)
	require.NoError(t, err)
	ctx = thread.NewTokenContext(ctx, tok)

	usage, err := client.GetUsage(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, usage.Usage)

	usage, err = client.GetUsage(ctx)
	require.NoError(t, err)
}

func TestAccountBuckets(t *testing.T) {
	conf, users, hub, threads, _, buckets := setup(t, nil)
	ctx := context.Background()

	// Signup, create an API key, and sign it for the requests
	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	devCtx := common.NewSessionContext(ctx, dev.Session)
	res, err := hub.CreateKey(devCtx, hubpb.KeyType_KEY_TYPE_ACCOUNT, true)
	require.NoError(t, err)
	ctx = common.NewAPIKeyContext(ctx, res.KeyInfo.Key)
	ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Minute), res.KeyInfo.Secret)
	require.NoError(t, err)

	// Create a db for the bucket
	ctx = common.NewThreadNameContext(ctx, "my-buckets")
	dbID := thread.NewIDV1(thread.Raw, 32)
	err = threads.NewDB(ctx, dbID)
	require.NoError(t, err)

	// Initialize a new bucket in the db
	ctx = common.NewThreadIDContext(ctx, dbID)
	buck, err := buckets.Create(ctx)
	require.NoError(t, err)

	// Finally, push a file to the bucket.
	res2, err := pushPath(t, ctx, buckets, buck.Root.Key, "file1.jpg", "testdata/file1.jpg")
	require.NoError(t, err)
	assert.NotEmpty(t, res2.Root.String())

	// We should have a thread named "my-buckets"
	res3, err := users.GetThread(ctx, "my-buckets")
	require.NoError(t, err)
	assert.Equal(t, dbID.Bytes(), res3.Id)
}

func TestUserBuckets(t *testing.T) {
	conf, users, hub, threads, net, buckets := setup(t, nil)
	ctx := context.Background()

	// Signup, create an API key, and sign it for the requests
	dev := apitest.Signup(t, hub, conf, apitest.NewUsername(), apitest.NewEmail())
	devCtx := common.NewSessionContext(ctx, dev.Session)
	res, err := hub.CreateKey(devCtx, hubpb.KeyType_KEY_TYPE_USER, true)
	require.NoError(t, err)
	ctx = common.NewAPIKeyContext(ctx, res.KeyInfo.Key)
	ctx, err = common.CreateAPISigContext(ctx, time.Now().Add(time.Hour), res.KeyInfo.Secret)
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
	buck, err := buckets.Create(ctx, bc.WithPrivate(true))
	require.NoError(t, err)

	// Finally, push a file to the bucket.
	res2, err := pushPath(t, ctx, buckets, buck.Root.Key, "file1.jpg", "testdata/file1.jpg")
	require.NoError(t, err)
	assert.NotEmpty(t, res2.Root.String())

	// We should have a thread named "my-buckets"
	res3, err := users.GetThread(ctx, "my-buckets")
	require.NoError(t, err)
	assert.Equal(t, dbID.Bytes(), res3.Id)

	// The dev should see that the key was used to create one thread
	keys, err := hub.ListKeys(devCtx)
	require.NoError(t, err)
	assert.Equal(t, 1, len(keys.List))
	assert.Equal(t, 1, int(keys.List[0].Threads))

	// Try interacting with the bucket as a different user
	sk2, pk2c, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	tok2, err := threads.GetToken(ctx, thread.NewLibp2pIdentity(sk2))
	require.NoError(t, err)
	ctx = thread.NewTokenContext(ctx, tok2)
	pk2 := thread.NewLibp2pPubKey(pk2c).String()

	// Try to overwrite existing
	_, err = pushPath(t, ctx, buckets, buck.Root.Key, "file1.jpg", "testdata/file2.jpg")
	require.Error(t, err) // no path write access

	// Try to add a new path
	_, err = pushPath(t, ctx, buckets, buck.Root.Key, "file2.jpg", "testdata/file2.jpg")
	require.Error(t, err) // no new path write access

	// Try to get the path
	_, err = buckets.ListPath(ctx, buck.Root.Key, "file1.jpg")
	require.Error(t, err) // no path read access

	// Grant path read access
	ctx = thread.NewTokenContext(ctx, tok)
	err = buckets.PushPathAccessRoles(ctx, buck.Root.Key, "file1.jpg", map[string]bucks.Role{
		pk2: bucks.Reader,
	})
	require.NoError(t, err)

	// Try to get the path again
	ctx = thread.NewTokenContext(ctx, tok2)
	_, err = buckets.ListPath(ctx, buck.Root.Key, "file1.jpg")
	require.NoError(t, err)

	// Grant path write access
	ctx = thread.NewTokenContext(ctx, tok)
	err = buckets.PushPathAccessRoles(ctx, buck.Root.Key, "file1.jpg", map[string]bucks.Role{
		pk2: bucks.Writer,
	})
	require.NoError(t, err)

	// Try to write to the path again
	ctx = thread.NewTokenContext(ctx, tok2)
	res4, err := pushPath(t, ctx, buckets, buck.Root.Key, "file1.jpg", "testdata/file2.jpg")
	require.NoError(t, err)
	assert.False(t, res4.Root.Cid().Equals(res2.Root.Cid()))

	// Check that we now have two logs in the bucket thread
	time.Sleep(time.Second)
	info, err := net.GetThread(ctx, dbID, corenet.WithThreadToken(tok))
	require.NoError(t, err)
	assert.Len(t, info.Logs, 2)
}

func setup(t *testing.T, conf *core.Config) (core.Config, *c.Client, *hc.Client, *tc.Client, *nc.Client, *bc.Client) {
	if conf == nil {
		tmp := apitest.DefaultTextileConfig(t)
		conf = &tmp
	}
	return setupWithConf(t, *conf)
}

func setupWithConf(t *testing.T, conf core.Config) (core.Config, *c.Client, *hc.Client, *tc.Client, *nc.Client, *bc.Client) {
	apitest.MakeTextileWithConfig(t, conf)
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

func setupWithBilling(t *testing.T) (core.Config, *c.Client, *hc.Client, *tc.Client, *nc.Client, *bc.Client) {
	bconf := apitest.DefaultBillingConfig(t)
	apitest.MakeBillingWithConfig(t, bconf)

	conf := apitest.DefaultTextileConfig(t)
	billingApi, err := tutil.TCPAddrFromMultiAddr(bconf.ListenAddr)
	require.NoError(t, err)
	conf.AddrBillingAPI = billingApi
	return setup(t, &conf)
}

func setupUserMail(t *testing.T, client *c.Client, threads *tc.Client, key string) (thread.Identity, context.Context) {
	ctx := common.NewAPIKeyContext(context.Background(), key)
	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	id := thread.NewLibp2pIdentity(sk)
	tok, err := threads.GetToken(ctx, id)
	require.NoError(t, err)
	ctx = thread.NewTokenContext(ctx, tok)
	_, err = client.SetupMailbox(ctx)
	require.NoError(t, err)
	return id, ctx
}

func pushPath(t *testing.T, ctx context.Context, buckets *bc.Client, key, pth, name string) (bc.PushPathsResult, error) {
	q, err := buckets.PushPaths(ctx, key)
	require.NoError(t, err)
	err = q.AddFile(pth, name)
	require.NoError(t, err)
	for q.Next() {
	}
	q.Close()
	return q.Current, q.Err()
}
