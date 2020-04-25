package client_test

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/apitest"
	c "github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	hc "github.com/textileio/textile/api/hub/client"
	"google.golang.org/grpc"
)

func TestClient_Init(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	buck, err := client.Init(ctx)
	require.Nil(t, err)
	assert.NotEmpty(t, buck.Root)
	assert.NotEmpty(t, buck.Root.Key)
	assert.NotEmpty(t, buck.Root.Path)
	assert.NotEmpty(t, buck.Root.CreatedAt)
	assert.NotEmpty(t, buck.Root.UpdatedAt)
}

func TestClient_List(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	t.Run("empty", func(t *testing.T) {
		rep, err := client.List(ctx)
		require.Nil(t, err)
		assert.Equal(t, 0, len(rep.Roots))
	})

	buck, err := client.Init(ctx)
	require.Nil(t, err)

	t.Run("not empty", func(t *testing.T) {
		rep, err := client.List(ctx)
		require.Nil(t, err)
		assert.Equal(t, 1, len(rep.Roots))
		assert.Equal(t, buck.Root.Key, rep.Roots[0].Key)
		assert.Equal(t, buck.Root.Path, rep.Roots[0].Path)
		assert.Equal(t, buck.Root.CreatedAt, rep.Roots[0].CreatedAt)
		assert.Equal(t, buck.Root.UpdatedAt, rep.Roots[0].UpdatedAt)
	})
}

func TestClient_ListPath(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	buck, err := client.Init(ctx)
	require.Nil(t, err)

	t.Run("empty", func(t *testing.T) {
		rep, err := client.ListPath(ctx, buck.Root.Key, "")
		require.Nil(t, err)
		assert.NotEmpty(t, rep.Root)
		assert.True(t, rep.Item.IsDir)
		assert.Equal(t, 0, len(rep.Item.Items))
	})

	file, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "dir1/file1.jpg", file)
	require.Nil(t, err)
	_, root, err := client.PushPath(ctx, buck.Root.Key, "dir2/file1.jpg", file)
	require.Nil(t, err)

	t.Run("root dir", func(t *testing.T) {
		rep, err := client.ListPath(ctx, buck.Root.Key, "")
		require.Nil(t, err)
		assert.True(t, rep.Item.IsDir)
		assert.Equal(t, 2, len(rep.Item.Items))
	})

	t.Run("nested dir", func(t *testing.T) {
		rep, err := client.ListPath(ctx, buck.Root.Key, "dir1")
		require.Nil(t, err)
		assert.True(t, rep.Item.IsDir)
		assert.Equal(t, 1, len(rep.Item.Items))
	})

	t.Run("file", func(t *testing.T) {
		rep, err := client.ListPath(ctx, buck.Root.Key, "dir1/file1.jpg")
		require.Nil(t, err)
		assert.True(t, strings.HasSuffix(rep.Item.Path, "file1.jpg"))
		assert.False(t, rep.Item.IsDir)
		assert.Equal(t, root.String(), rep.Root.Path)
	})
}

func TestClient_PushPath(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	buck, err := client.Init(ctx)
	require.Nil(t, err)

	t.Run("bucket path", func(t *testing.T) {
		file, err := os.Open("testdata/file1.jpg")
		require.Nil(t, err)
		defer file.Close()
		progress := make(chan int64)
		go func() {
			for p := range progress {
				t.Logf("progress: %d", p)
			}
		}()
		pth, root, err := client.PushPath(ctx, buck.Root.Key, "file1.jpg", file, c.WithProgress(progress))
		require.Nil(t, err)
		assert.NotEmpty(t, pth)
		assert.NotEmpty(t, root)
	})

	t.Run("nested bucket path", func(t *testing.T) {
		file, err := os.Open("testdata/file2.jpg")
		require.Nil(t, err)
		defer file.Close()
		progress := make(chan int64)
		go func() {
			for p := range progress {
				t.Logf("progress: %d", p)
			}
		}()
		pth, root, err := client.PushPath(ctx, buck.Root.Key, "path/to/file2.jpg", file, c.WithProgress(progress))
		require.Nil(t, err)
		assert.NotEmpty(t, pth)
		assert.NotEmpty(t, root)

		rep, err := client.ListPath(ctx, buck.Root.Key, "")
		require.Nil(t, err)
		assert.Equal(t, 2, len(rep.Item.Items))
	})
}

func TestClient_PullPath(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	buck, err := client.Init(ctx)
	require.Nil(t, err)

	file, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "file1.jpg", file)
	require.Nil(t, err)

	t.Run("bucket path", func(t *testing.T) {
		file, err := ioutil.TempFile("", "")
		require.Nil(t, err)
		defer file.Close()

		progress := make(chan int64)
		go func() {
			for p := range progress {
				t.Logf("progress: %d", p)
			}
		}()
		err = client.PullPath(ctx, buck.Root.Key, "file1.jpg", file, c.WithProgress(progress))
		require.Nil(t, err)
		info, err := file.Stat()
		require.Nil(t, err)
		t.Logf("wrote file with size %d", info.Size())
	})
}

func TestClient_Remove(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	buck, err := client.Init(ctx)
	require.Nil(t, err)

	file1, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file1.Close()
	file2, err := os.Open("testdata/file2.jpg")
	require.Nil(t, err)
	defer file2.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "file1.jpg", file1)
	require.Nil(t, err)
	_, _, err = client.PushPath(ctx, buck.Root.Key, "again/file2.jpg", file1)
	require.Nil(t, err)

	err = client.Remove(ctx, buck.Root.Key)
	require.Nil(t, err)
	_, err = client.ListPath(ctx, buck.Root.Key, "again/file2.jpg")
	require.NotNil(t, err)
}

func TestClient_RemovePath(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	buck, err := client.Init(ctx)
	require.Nil(t, err)

	file1, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file1.Close()
	file2, err := os.Open("testdata/file2.jpg")
	require.Nil(t, err)
	defer file2.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "file1.jpg", file1)
	require.Nil(t, err)
	_, _, err = client.PushPath(ctx, buck.Root.Key, "again/file2.jpg", file1)
	require.Nil(t, err)

	err = client.RemovePath(ctx, buck.Root.Key, "again/file2.jpg")
	require.Nil(t, err)
	_, err = client.ListPath(ctx, buck.Root.Key, "again/file2.jpg")
	require.NotNil(t, err)
	rep, err := client.ListPath(ctx, buck.Root.Key, "")
	require.Nil(t, err)
	assert.Equal(t, 2, len(rep.Item.Items))

	err = client.RemovePath(ctx, buck.Root.Key, "again")
	require.Nil(t, err)
	_, err = client.ListPath(ctx, buck.Root.Key, "again")
	require.NotNil(t, err)
	rep, err = client.ListPath(ctx, buck.Root.Key, "")
	require.Nil(t, err)
	assert.Equal(t, 1, len(rep.Item.Items))
}

func TestClose(t *testing.T) {
	t.Parallel()
	conf, shutdown := apitest.MakeTextile(t)
	defer shutdown()
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrApi)
	require.Nil(t, err)
	client, err := c.NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{}))
	require.Nil(t, err)

	t.Run("test close", func(t *testing.T) {
		err := client.Close()
		require.Nil(t, err)
	})
}

func setup(t *testing.T) (context.Context, *c.Client, func()) {
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

	user := apitest.Signup(t, hubclient, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	id := thread.NewIDV1(thread.Raw, 32)
	ctx = common.NewThreadNameContext(ctx, "buckets")
	err = threadsclient.NewDB(ctx, id)
	require.Nil(t, err)
	ctx = common.NewThreadIDContext(ctx, id)

	return ctx, client, func() {
		shutdown()
		err := client.Close()
		require.Nil(t, err)
	}
}
