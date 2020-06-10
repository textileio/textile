package client_test

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/ipfs/interface-go-ipfs-core/path"

	ipfsfiles "github.com/ipfs/go-ipfs-files"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/apitest"
	c "github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	hc "github.com/textileio/textile/api/hub/client"
	"github.com/textileio/textile/util"
	"google.golang.org/grpc"
)

func TestClient_Init(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	buck, err := client.Init(ctx, "mybuck")
	require.Nil(t, err)
	assert.NotEmpty(t, buck.Root)
	assert.NotEmpty(t, buck.Root.Key)
	assert.NotEmpty(t, buck.Root.Path)
	assert.NotEmpty(t, buck.Root.CreatedAt)
	assert.NotEmpty(t, buck.Root.UpdatedAt)
	assert.NotEmpty(t, buck.Links)
	assert.NotEmpty(t, buck.Seed)
}

func TestClient_Root(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	buck, err := client.Init(ctx, "mybuck")
	require.Nil(t, err)

	root, err := client.Root(ctx, buck.Root.Key)
	require.Nil(t, err)
	assert.NotEmpty(t, root.Root)
	assert.NotEmpty(t, root.Root.Key)
	assert.NotEmpty(t, root.Root.Path)
	assert.NotEmpty(t, root.Root.CreatedAt)
	assert.NotEmpty(t, root.Root.UpdatedAt)
}

func TestClient_Links(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	buck, err := client.Init(ctx, "mybuck")
	require.Nil(t, err)

	links, err := client.Links(ctx, buck.Root.Key)
	require.Nil(t, err)
	assert.NotEmpty(t, links.URL)
	assert.NotEmpty(t, links.IPNS)
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

	buck, err := client.Init(ctx, "mybuck")
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

	buck, err := client.Init(ctx, "mybuck")
	require.Nil(t, err)

	t.Run("empty", func(t *testing.T) {
		rep, err := client.ListPath(ctx, buck.Root.Key, "")
		require.Nil(t, err)
		assert.NotEmpty(t, rep.Root)
		assert.True(t, rep.Item.IsDir)
		assert.Equal(t, 1, len(rep.Item.Items))
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
		assert.Equal(t, 3, len(rep.Item.Items))
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

func TestClient_SetPath(t *testing.T) {
	t.Parallel()
	innerPaths := []struct {
		Name string
		Path string
		// How many files should be expected at the bucket root
		// in the first SetHead.
		NumFilesAtRootFirst int
		// How many files should be expected at the *imported* Cid
		// level.
		NumFilesAtImportedLevelFirst int
		// How many files should be expected at the bucket root
		// in the second SetHead.
		NumFilesAtRootSecond int
		// How many files should be expected at the *imported* Cid
		// level.
		NumFilesAtImportedLevelSecond int
	}{
		{Name: "nested",
			Path: "nested",
			// At root level, .seed and nested dir.
			NumFilesAtRootFirst: 2,
			// The first SetHead has one file, and one dir.
			NumFilesAtImportedLevelFirst: 2,
			NumFilesAtRootSecond:         2,
			// The second SetHead only has one file.
			NumFilesAtImportedLevelSecond: 1,
		},
		// Edge case, Path is empty. So "AtRoot" or "AtImportedLevel"
		// is the same.
		{Name: "root",
			Path:                         "",
			NumFilesAtRootFirst:          3,
			NumFilesAtImportedLevelFirst: 3,
			// In both below cases, the files are the .seed
			// and the fileVersion2.jpg.
			NumFilesAtRootSecond:          2,
			NumFilesAtImportedLevelSecond: 2,
		},
	}

	for _, innerPath := range innerPaths {
		t.Run(innerPath.Name, func(t *testing.T) {
			ctx, client, done := setup(t)
			defer done()

			file1, err := os.Open("testdata/file1.jpg")
			require.Nil(t, err)
			defer file1.Close()
			file2, err := os.Open("testdata/file2.jpg")
			require.Nil(t, err)
			defer file2.Close()

			ipfs, err := httpapi.NewApi(util.MustParseAddr("/ip4/127.0.0.1/tcp/5001"))
			require.NoError(t, err)
			p, err := ipfs.Unixfs().Add(
				ctx,
				ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
					"file1.jpg": ipfsfiles.NewReaderFile(file1),
					"folder1": ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
						"file2.jpg": ipfsfiles.NewReaderFile(file2),
					}),
				}),
			)
			require.NoError(t, err)

			buck, err := client.Init(ctx, "mybuck")
			require.NoError(t, err)

			_, err = client.SetPath(ctx, buck.Root.Key, innerPath.Path, p.Cid())
			require.NoError(t, err)

			rep, err := client.ListPath(ctx, buck.Root.Key, "")
			require.Nil(t, err)
			assert.True(t, rep.Item.IsDir)
			assert.Len(t, rep.Item.Items, innerPath.NumFilesAtRootFirst)

			rep, err = client.ListPath(ctx, buck.Root.Key, innerPath.Path)
			require.Nil(t, err)
			assert.True(t, rep.Item.IsDir)
			assert.Len(t, rep.Item.Items, innerPath.NumFilesAtImportedLevelFirst)

			// SetPath again in the same path, but with a different dag.
			// Should replace what was already under that path.
			file1, err = os.Open("testdata/file1.jpg")
			require.Nil(t, err)
			defer file1.Close()
			p, err = ipfs.Unixfs().Add(
				ctx,
				ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
					"fileVersion2.jpg": ipfsfiles.NewReaderFile(file1),
				}),
			)
			require.NoError(t, err)
			_, err = client.SetPath(ctx, buck.Root.Key, innerPath.Path, p.Cid())
			require.NoError(t, err)

			rep, err = client.ListPath(ctx, buck.Root.Key, "")
			require.Nil(t, err)
			assert.True(t, rep.Item.IsDir)
			assert.Len(t, rep.Item.Items, innerPath.NumFilesAtRootSecond)

			rep, err = client.ListPath(ctx, buck.Root.Key, innerPath.Path)
			require.Nil(t, err)
			assert.True(t, rep.Item.IsDir)
			assert.Len(t, rep.Item.Items, innerPath.NumFilesAtImportedLevelSecond)

		})
	}
}

func TestClient_ListIpfsPath(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	_ = client
	defer done()

	file1, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file1.Close()
	file2, err := os.Open("testdata/file2.jpg")
	require.Nil(t, err)
	defer file2.Close()

	ipfs, err := httpapi.NewApi(util.MustParseAddr("/ip4/127.0.0.1/tcp/5001"))
	require.NoError(t, err)
	p, err := ipfs.Unixfs().Add(
		ctx,
		ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
			"file1.jpg": ipfsfiles.NewReaderFile(file1),
			"folder1": ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
				"file2.jpg": ipfsfiles.NewReaderFile(file2),
			}),
		}),
	)
	require.NoError(t, err)

	r, err := client.ListIpfsPath(ctx, p)
	require.NoError(t, err)
	require.True(t, r.Item.IsDir)
	require.Len(t, r.Item.Items, 2)
}

func TestClient_PullIpfsPath(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	_ = client
	defer done()

	file1, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file1.Close()
	file2, err := os.Open("testdata/file2.jpg")
	require.Nil(t, err)
	defer file2.Close()

	ipfs, err := httpapi.NewApi(util.MustParseAddr("/ip4/127.0.0.1/tcp/5001"))
	require.NoError(t, err)
	p, err := ipfs.Unixfs().Add(
		ctx,
		ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
			"file1.jpg": ipfsfiles.NewReaderFile(file1),
			"folder1": ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
				"file2.jpg": ipfsfiles.NewReaderFile(file2),
			}),
		}),
	)
	require.NoError(t, err)

	tmpFile, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer tmpFile.Close()
	defer func() { os.Remove(tmpFile.Name()) }()
	tmpName := tmpFile.Name()

	err = client.PullIpfsPath(ctx, path.Join(p, "folder1/file2.jpg"), tmpFile)
	require.NoError(t, err)
	tmpFile.Close()

	file2, err = os.Open("testdata/file2.jpg")
	require.NoError(t, err)
	defer file2.Close()
	origBytes, err := ioutil.ReadAll(file2)
	require.NoError(t, err)
	tmpFile, err = os.Open(tmpName)
	require.NoError(t, err)
	defer tmpFile.Close()
	tmpBytes, err := ioutil.ReadAll(tmpFile)
	require.NoError(t, err)
	require.True(t, bytes.Equal(origBytes, tmpBytes))
}

func TestClient_InitWithCid(t *testing.T) {

	t.Parallel()
	ctx, client, done := setup(t)
	_ = client
	defer done()

	file1, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file1.Close()
	file2, err := os.Open("testdata/file2.jpg")
	require.Nil(t, err)
	defer file2.Close()

	ipfs, err := httpapi.NewApi(util.MustParseAddr("/ip4/127.0.0.1/tcp/5001"))
	require.NoError(t, err)
	p, err := ipfs.Unixfs().Add(
		ctx,
		ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
			"file1.jpg": ipfsfiles.NewReaderFile(file1),
			"folder1": ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
				"file2.jpg": ipfsfiles.NewReaderFile(file2),
			}),
		}),
	)
	require.NoError(t, err)
	initCid := p.Cid()
	buck, err := client.Init(ctx, "mybuck", c.WithCid(initCid))
	require.NoError(t, err)

	// Assert top level bucket.
	rep, err := client.ListPath(ctx, buck.Root.Key, "")
	require.Nil(t, err)
	assert.True(t, rep.Item.IsDir)
	topLevelNames := make([]string, 3)
	for i, n := range rep.Item.Items {
		topLevelNames[i] = n.Name
	}
	assert.Contains(t, topLevelNames, "file1.jpg")
	assert.Contains(t, topLevelNames, "folder1")

	// Assert inner directory.
	rep, err = client.ListPath(ctx, buck.Root.Key, "folder1")
	require.Nil(t, err)
	assert.True(t, rep.Item.IsDir)
	assert.Equal(t, 1, len(rep.Item.Items))
	assert.Equal(t, rep.Item.Items[0].Name, "file2.jpg")
}

func TestClient_PushPath(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	buck, err := client.Init(ctx, "mybuck")
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
		assert.Equal(t, 3, len(rep.Item.Items))
	})
}

func TestClient_PullPath(t *testing.T) {
	t.Parallel()
	ctx, client, done := setup(t)
	defer done()

	buck, err := client.Init(ctx, "mybuck")
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

	buck, err := client.Init(ctx, "mybuck")
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

	buck, err := client.Init(ctx, "mybuck")
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

	pth, err := client.RemovePath(ctx, buck.Root.Key, "again/file2.jpg")
	require.Nil(t, err)
	assert.NotEmpty(t, pth)
	_, err = client.ListPath(ctx, buck.Root.Key, "again/file2.jpg")
	require.NotNil(t, err)
	rep, err := client.ListPath(ctx, buck.Root.Key, "")
	require.Nil(t, err)
	assert.Equal(t, 3, len(rep.Item.Items))

	_, err = client.RemovePath(ctx, buck.Root.Key, "again")
	require.Nil(t, err)
	_, err = client.ListPath(ctx, buck.Root.Key, "again")
	require.NotNil(t, err)
	rep, err = client.ListPath(ctx, buck.Root.Key, "")
	require.Nil(t, err)
	assert.Equal(t, 2, len(rep.Item.Items))
}

func TestClose(t *testing.T) {
	t.Parallel()
	conf, shutdown := apitest.MakeTextile(t)
	defer shutdown()
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
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
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
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
