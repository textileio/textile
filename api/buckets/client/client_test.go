package client_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"testing"

	ipfsfiles "github.com/ipfs/go-ipfs-files"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/apitest"
	"github.com/textileio/textile/api/buckets"
	c "github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/api/common"
	hc "github.com/textileio/textile/api/hub/client"
	bucks "github.com/textileio/textile/buckets"
	"github.com/textileio/textile/core"
	"github.com/textileio/textile/util"
	"google.golang.org/grpc"
)

func TestClient_Create(t *testing.T) {
	ctx, client := setup(t)

	buck, err := client.Create(ctx, c.WithName("mybuck"))
	require.NoError(t, err)
	assert.NotEmpty(t, buck.Root)
	assert.NotEmpty(t, buck.Root.Key)
	assert.NotEmpty(t, buck.Root.Name)
	assert.NotEmpty(t, buck.Root.Path)
	assert.NotEmpty(t, buck.Root.CreatedAt)
	assert.NotEmpty(t, buck.Root.UpdatedAt)
	assert.NotEmpty(t, buck.Links)
	assert.NotEmpty(t, buck.Seed)

	pbuck, err := client.Create(ctx, c.WithName("myprivatebuck"), c.WithPrivate(true))
	require.NoError(t, err)
	assert.NotEmpty(t, pbuck.Root)
	assert.NotEmpty(t, pbuck.Root.Key)
	assert.NotEmpty(t, pbuck.Root.Name)
	assert.NotEmpty(t, pbuck.Root.Path)
	assert.NotEmpty(t, pbuck.Root.CreatedAt)
	assert.NotEmpty(t, pbuck.Root.UpdatedAt)
	assert.NotEmpty(t, pbuck.Links)
	assert.NotEmpty(t, pbuck.Seed)
}

func TestClient_InitExceedLimit(t *testing.T) {
	conf := apitest.DefaultTextileConfig(t)
	conf.BucketsMaxNumberPerThread = 1
	ctx, client := setupWithConf(t, conf)

	_, err := client.Create(ctx, c.WithName("mybuck"))
	require.NoError(t, err)
	_, err = client.Create(ctx, c.WithName("myprivatebuck"), c.WithPrivate(true))
	require.Error(t, err)
	require.Contains(t, err.Error(), buckets.ErrTooManyBucketsInThread.Error())
}

func TestClient_InitWithCid(t *testing.T) {
	ctx, client := setup(t)

	t.Run("public", func(t *testing.T) {
		initWithCid(t, ctx, client, false)
	})

	t.Run("private", func(t *testing.T) {
		initWithCid(t, ctx, client, true)
	})
}

func initWithCid(t *testing.T, ctx context.Context, client *c.Client, private bool) {
	file1, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file1.Close()
	file2, err := os.Open("testdata/file2.jpg")
	require.NoError(t, err)
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
	buck, err := client.Create(ctx, c.WithCid(initCid), c.WithPrivate(private))
	require.NoError(t, err)

	// Assert top level bucket.
	rep, err := client.ListPath(ctx, buck.Root.Key, "")
	require.NoError(t, err)
	assert.True(t, rep.Item.IsDir)
	topLevelNames := make([]string, 3)
	for i, n := range rep.Item.Items {
		topLevelNames[i] = n.Name
	}
	assert.Contains(t, topLevelNames, "file1.jpg")
	assert.Contains(t, topLevelNames, "folder1")

	// Assert inner directory.
	rep, err = client.ListPath(ctx, buck.Root.Key, "folder1")
	require.NoError(t, err)
	assert.True(t, rep.Item.IsDir)
	assert.Len(t, rep.Item.Items, 1)
	assert.Equal(t, rep.Item.Items[0].Name, "file2.jpg")
}

func TestClient_Root(t *testing.T) {
	ctx, client := setup(t)

	buck, err := client.Create(ctx)
	require.NoError(t, err)

	root, err := client.Root(ctx, buck.Root.Key)
	require.NoError(t, err)
	assert.NotEmpty(t, root.Root)
	assert.NotEmpty(t, root.Root.Key)
	assert.NotEmpty(t, root.Root.Owner)
	assert.NotEmpty(t, root.Root.Version)
	assert.NotEmpty(t, root.Root.Path)
	assert.NotEmpty(t, root.Root.CreatedAt)
	assert.NotEmpty(t, root.Root.UpdatedAt)
}

func TestClient_Links(t *testing.T) {
	ctx, client := setup(t)

	buck, err := client.Create(ctx)
	require.NoError(t, err)

	links, err := client.Links(ctx, buck.Root.Key)
	require.NoError(t, err)
	assert.NotEmpty(t, links.Url)
	assert.NotEmpty(t, links.Ipns)
}

func TestClient_List(t *testing.T) {
	ctx, client := setup(t)

	t.Run("empty", func(t *testing.T) {
		rep, err := client.List(ctx)
		require.NoError(t, err)
		assert.Len(t, rep.Roots, 0)
	})

	buck, err := client.Create(ctx)
	require.NoError(t, err)

	t.Run("not empty", func(t *testing.T) {
		rep, err := client.List(ctx)
		require.NoError(t, err)
		assert.Len(t, rep.Roots, 1)
		assert.Equal(t, buck.Root.Key, rep.Roots[0].Key)
		assert.Equal(t, buck.Root.Path, rep.Roots[0].Path)
		assert.Equal(t, buck.Root.CreatedAt, rep.Roots[0].CreatedAt)
		assert.Equal(t, buck.Root.UpdatedAt, rep.Roots[0].UpdatedAt)
	})
}

func TestClient_ListPath(t *testing.T) {
	ctx, client := setup(t)

	t.Run("public", func(t *testing.T) {
		listPath(t, ctx, client, false)
	})

	t.Run("private", func(t *testing.T) {
		listPath(t, ctx, client, true)
	})
}

func listPath(t *testing.T, ctx context.Context, client *c.Client, private bool) {
	buck, err := client.Create(ctx, c.WithPrivate(private))
	require.NoError(t, err)

	t.Run("empty", func(t *testing.T) {
		rep, err := client.ListPath(ctx, buck.Root.Key, "")
		require.NoError(t, err)
		assert.NotEmpty(t, rep.Root)
		assert.True(t, rep.Item.IsDir)
		assert.Len(t, rep.Item.Items, 1)
	})

	file, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "dir1/file1.jpg", file)
	require.NoError(t, err)
	_, root, err := client.PushPath(ctx, buck.Root.Key, "dir2/file1.jpg", file)
	require.NoError(t, err)

	t.Run("root dir", func(t *testing.T) {
		rep, err := client.ListPath(ctx, buck.Root.Key, "")
		require.NoError(t, err)
		assert.True(t, rep.Item.IsDir)
		assert.Len(t, rep.Item.Items, 3)
		dir1i := sort.Search(len(rep.Item.Items), func(i int) bool {
			return rep.Item.Items[i].Name == "dir1"
		})
		assert.True(t, dir1i < len(rep.Item.Items))
		assert.True(t, rep.Item.Items[dir1i].IsDir)
	})

	t.Run("nested dir", func(t *testing.T) {
		rep, err := client.ListPath(ctx, buck.Root.Key, "dir1")
		require.NoError(t, err)
		assert.True(t, rep.Item.IsDir)
		assert.Len(t, rep.Item.Items, 1)
	})

	t.Run("file", func(t *testing.T) {
		rep, err := client.ListPath(ctx, buck.Root.Key, "dir1/file1.jpg")
		require.NoError(t, err)
		assert.True(t, strings.HasSuffix(rep.Item.Path, "file1.jpg"))
		assert.False(t, rep.Item.IsDir)
		assert.Equal(t, root.String(), rep.Root.Path)
	})
}

func TestClient_ListIpfsPath(t *testing.T) {
	ctx, client := setup(t)

	file1, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file1.Close()
	file2, err := os.Open("testdata/file2.jpg")
	require.NoError(t, err)
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

func TestClient_PushPath(t *testing.T) {
	ctx, client := setup(t)

	t.Run("public", func(t *testing.T) {
		pushPath(t, ctx, client, false)
	})

	t.Run("private", func(t *testing.T) {
		pushPath(t, ctx, client, true)
	})
}

func TestClient_PushPathBucketExceedLimit(t *testing.T) {
	firstFile := "testdata/file1.jpg"
	file1, err := os.Open(firstFile)
	require.NoError(t, err)
	defer file1.Close()
	stat, err := os.Stat(firstFile)
	require.NoError(t, err)

	// Calculate a max-size which lets the first file
	// be accepted, with a bit of margin. A second file
	// will fail since it exceeded the bucket limit.
	maxBucketSize := stat.Size() + 1024

	conf := apitest.DefaultTextileConfig(t)
	conf.BucketsMaxSize = maxBucketSize
	ctx, client := setupWithConf(t, conf)

	buck, err := client.Create(ctx)
	require.NoError(t, err)

	pth1, root1, err := client.PushPath(ctx, buck.Root.Key, "file1.jpg", file1)
	require.NoError(t, err)
	assert.NotEmpty(t, pth1)
	assert.NotEmpty(t, root1)

	file2, err := os.Open("testdata/file2.jpg")
	require.NoError(t, err)
	defer file2.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "path/to/file2.jpg", file2)
	require.Error(t, err)
	require.Contains(t, err.Error(), buckets.ErrBucketExceedsMaxSize.Error())
}

func TestClient_PushPathBucketsExceedLimit(t *testing.T) {
	firstFile := "testdata/file1.jpg"
	file1, err := os.Open(firstFile)
	require.NoError(t, err)
	defer file1.Close()
	stat, err := os.Stat(firstFile)
	require.NoError(t, err)

	maxBucketsSize := stat.Size() + 1024

	conf := apitest.DefaultTextileConfig(t)
	conf.BucketsTotalMaxSize = maxBucketsSize
	ctx, client := setupWithConf(t, conf)

	buck, err := client.Create(ctx)
	require.NoError(t, err)

	pth1, root1, err := client.PushPath(ctx, buck.Root.Key, "file1.jpg", file1)
	require.NoError(t, err)
	assert.NotEmpty(t, pth1)
	assert.NotEmpty(t, root1)

	// Create a second bucket. Since the limit is bucket-wide
	// should fail.
	buck, err = client.Create(ctx)
	require.NoError(t, err)

	file2, err := os.Open("testdata/file2.jpg")
	require.NoError(t, err)
	defer file2.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "path/to/file2.jpg", file2)
	require.Error(t, err)
	require.Contains(t, err.Error(), buckets.ErrBucketsTotalSizeExceedsMaxSize.Error())
}

func TestClient_PushPathPrivateBucketsExceedLimit(t *testing.T) {
	firstFile := "testdata/file1.jpg"
	file1, err := os.Open(firstFile)
	require.NoError(t, err)
	defer file1.Close()
	stat, err := os.Stat(firstFile)
	require.NoError(t, err)

	maxBucketsSize := stat.Size() + 1024

	conf := apitest.DefaultTextileConfig(t)
	conf.BucketsTotalMaxSize = maxBucketsSize
	ctx, client := setupWithConf(t, conf)

	buck, err := client.Create(ctx, c.WithPrivate(true))
	require.NoError(t, err)

	pth1, root1, err := client.PushPath(ctx, buck.Root.Key, "file1.jpg", file1)
	require.NoError(t, err)
	assert.NotEmpty(t, pth1)
	assert.NotEmpty(t, root1)

	// Create a second bucket. Since the limit is bucket-wide
	// should fail.
	buck, err = client.Create(ctx)
	require.NoError(t, err)

	file2, err := os.Open("testdata/file2.jpg")
	require.NoError(t, err)
	defer file2.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "path/to/file2.jpg", file2)
	require.Error(t, err)
	require.Contains(t, err.Error(), buckets.ErrBucketsTotalSizeExceedsMaxSize.Error())
}

func pushPath(t *testing.T, ctx context.Context, client *c.Client, private bool) {
	buck, err := client.Create(ctx, c.WithPrivate(private))
	require.NoError(t, err)

	file1, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file1.Close()
	progress1 := make(chan int64)
	go func() {
		for p := range progress1 {
			t.Logf("progress: %d", p)
		}
	}()
	pth1, root1, err := client.PushPath(ctx, buck.Root.Key, "file1.jpg", file1, c.WithProgress(progress1))
	require.NoError(t, err)
	assert.NotEmpty(t, pth1)
	assert.NotEmpty(t, root1)

	file2, err := os.Open("testdata/file2.jpg")
	require.NoError(t, err)
	defer file2.Close()
	progress2 := make(chan int64)
	go func() {
		for p := range progress2 {
			t.Logf("progress: %d", p)
		}
	}()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "path/to/file2.jpg", file2, c.WithProgress(progress2))
	require.NoError(t, err)

	rep1, err := client.ListPath(ctx, buck.Root.Key, "")
	require.NoError(t, err)
	assert.Len(t, rep1.Item.Items, 3)

	// Try overwriting the path
	_, _, err = client.PushPath(ctx, buck.Root.Key, "path/to/file2.jpg", strings.NewReader("seeya!"))
	require.NoError(t, err)

	rep2, err := client.ListPath(ctx, buck.Root.Key, "")
	require.NoError(t, err)
	assert.Len(t, rep2.Item.Items, 3)

	// Overwrite the path again, this time replacing a file link with a dir link
	file3, err := os.Open("testdata/file2.jpg")
	require.NoError(t, err)
	defer file3.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "path/to", file3)
	require.NoError(t, err)

	rep3, err := client.ListPath(ctx, buck.Root.Key, "")
	require.NoError(t, err)
	assert.Len(t, rep3.Item.Items, 3)
}

func TestClient_PullPath(t *testing.T) {
	ctx, client := setup(t)

	t.Run("public", func(t *testing.T) {
		pullPath(t, ctx, client, false)
	})

	t.Run("private", func(t *testing.T) {
		pullPath(t, ctx, client, true)
	})
}

func pullPath(t *testing.T, ctx context.Context, client *c.Client, private bool) {
	buck, err := client.Create(ctx, c.WithPrivate(private))
	require.NoError(t, err)

	file, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "file1.jpg", file)
	require.NoError(t, err)

	tmp, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer tmp.Close()

	progress := make(chan int64)
	go func() {
		for p := range progress {
			t.Logf("progress: %d", p)
		}
	}()
	err = client.PullPath(ctx, buck.Root.Key, "file1.jpg", tmp, c.WithProgress(progress))
	require.NoError(t, err)
	info, err := tmp.Stat()
	require.NoError(t, err)
	t.Logf("wrote file with size %d", info.Size())

	note := "baps!"
	_, _, err = client.PushPath(ctx, buck.Root.Key, "one/two/note.txt", strings.NewReader(note))
	require.NoError(t, err)

	var buf bytes.Buffer
	err = client.PullPath(ctx, buck.Root.Key, "one/two/note.txt", &buf)
	require.NoError(t, err)
	assert.Equal(t, note, buf.String())
}

func TestClient_PullIpfsPath(t *testing.T) {
	ctx, client := setup(t)

	file1, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file1.Close()
	file2, err := os.Open("testdata/file2.jpg")
	require.NoError(t, err)
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
	defer func() { _ = os.Remove(tmpFile.Name()) }()
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

func TestClient_SetPath(t *testing.T) {
	t.Run("public", func(t *testing.T) {
		setPath(t, false)
	})

	t.Run("private", func(t *testing.T) {
		setPath(t, true)
	})
}

func setPath(t *testing.T, private bool) {
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
		{
			Name: "nested",
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
		{
			Name:                         "root",
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
			ctx, client := setup(t)

			file1, err := os.Open("testdata/file1.jpg")
			require.NoError(t, err)
			defer file1.Close()
			file2, err := os.Open("testdata/file2.jpg")
			require.NoError(t, err)
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

			buck, err := client.Create(ctx, c.WithPrivate(private))
			require.NoError(t, err)

			_, err = client.SetPath(ctx, buck.Root.Key, innerPath.Path, p.Cid())
			require.NoError(t, err)

			rep, err := client.ListPath(ctx, buck.Root.Key, "")
			require.NoError(t, err)
			assert.True(t, rep.Item.IsDir)
			assert.Len(t, rep.Item.Items, innerPath.NumFilesAtRootFirst)

			rep, err = client.ListPath(ctx, buck.Root.Key, innerPath.Path)
			require.NoError(t, err)
			assert.True(t, rep.Item.IsDir)
			assert.Len(t, rep.Item.Items, innerPath.NumFilesAtImportedLevelFirst)

			// SetPath again in the same path, but with a different dag.
			// Should replace what was already under that path.
			file1, err = os.Open("testdata/file1.jpg")
			require.NoError(t, err)
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
			require.NoError(t, err)
			assert.True(t, rep.Item.IsDir)
			assert.Len(t, rep.Item.Items, innerPath.NumFilesAtRootSecond)

			rep, err = client.ListPath(ctx, buck.Root.Key, innerPath.Path)
			require.NoError(t, err)
			assert.True(t, rep.Item.IsDir)
			assert.Len(t, rep.Item.Items, innerPath.NumFilesAtImportedLevelSecond)
		})
	}
}

func TestClient_Remove(t *testing.T) {
	ctx, client := setup(t)

	t.Run("public", func(t *testing.T) {
		remove(t, ctx, client, false)
	})

	t.Run("private", func(t *testing.T) {
		remove(t, ctx, client, true)
	})
}

func remove(t *testing.T, ctx context.Context, client *c.Client, private bool) {
	buck, err := client.Create(ctx, c.WithPrivate(private))
	require.NoError(t, err)

	file1, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file1.Close()
	file2, err := os.Open("testdata/file2.jpg")
	require.NoError(t, err)
	defer file2.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "file1.jpg", file1)
	require.NoError(t, err)
	_, _, err = client.PushPath(ctx, buck.Root.Key, "again/file2.jpg", file2)
	require.NoError(t, err)

	err = client.Remove(ctx, buck.Root.Key)
	require.NoError(t, err)
	_, err = client.ListPath(ctx, buck.Root.Key, "again/file2.jpg")
	require.Error(t, err)
}

func TestClient_RemovePath(t *testing.T) {
	ctx, client := setup(t)

	t.Run("public", func(t *testing.T) {
		removePath(t, ctx, client, false)
	})

	t.Run("private", func(t *testing.T) {
		removePath(t, ctx, client, true)
	})
}

func removePath(t *testing.T, ctx context.Context, client *c.Client, private bool) {
	buck, err := client.Create(ctx, c.WithPrivate(private))
	require.NoError(t, err)

	file1, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file1.Close()
	file2, err := os.Open("testdata/file2.jpg")
	require.NoError(t, err)
	defer file2.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "file1.jpg", file1)
	require.NoError(t, err)
	_, _, err = client.PushPath(ctx, buck.Root.Key, "again/file2.jpg", file1)
	require.NoError(t, err)

	pth, err := client.RemovePath(ctx, buck.Root.Key, "again/file2.jpg")
	require.NoError(t, err)
	assert.NotEmpty(t, pth)
	_, err = client.ListPath(ctx, buck.Root.Key, "again/file2.jpg")
	require.Error(t, err)
	rep, err := client.ListPath(ctx, buck.Root.Key, "")
	require.NoError(t, err)
	assert.Len(t, rep.Item.Items, 2)

	_, err = client.RemovePath(ctx, buck.Root.Key, "again")
	require.NoError(t, err)
	_, err = client.ListPath(ctx, buck.Root.Key, "again")
	require.Error(t, err)
	rep, err = client.ListPath(ctx, buck.Root.Key, "")
	require.NoError(t, err)
	assert.Len(t, rep.Item.Items, 2)
}

func TestClient_GetPathAccessRoles(t *testing.T) {
	ctx, client := setup(t)

	buck, err := client.Create(ctx)
	require.NoError(t, err)
	file, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "file1.jpg", file)
	require.NoError(t, err)

	roles, err := client.GetPathAccessRoles(ctx, buck.Root.Key, "file1.jpg")
	require.NoError(t, err)
	assert.Len(t, roles, 2)
}

func TestClient_EditPathAccessRoles(t *testing.T) {
	ctx, client := setup(t)

	buck, err := client.Create(ctx)
	require.NoError(t, err)

	_, pk, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	reader := thread.NewLibp2pPubKey(pk)
	roles := map[string]bucks.Role{
		reader.String(): bucks.Reader,
	}
	err = client.EditPathAccessRoles(ctx, buck.Root.Key, "nothing/here", roles)
	require.Error(t, err)

	file, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file.Close()
	_, _, err = client.PushPath(ctx, buck.Root.Key, "file1.jpg", file)
	require.NoError(t, err)

	err = client.EditPathAccessRoles(ctx, buck.Root.Key, "file1.jpg", roles)
	require.NoError(t, err)
}

func TestClose(t *testing.T) {
	conf := apitest.MakeTextile(t)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.NoError(t, err)
	client, err := c.NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{}))
	require.NoError(t, err)

	t.Run("test close", func(t *testing.T) {
		err := client.Close()
		require.NoError(t, err)
	})
}

func setup(t *testing.T) (context.Context, *c.Client) {
	conf := apitest.DefaultTextileConfig(t)
	return setupWithConf(t, conf)
}

func setupWithConf(t *testing.T, conf core.Config) (context.Context, *c.Client) {
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

	user := apitest.Signup(t, hubclient, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	id := thread.NewIDV1(thread.Raw, 32)
	ctx = common.NewThreadNameContext(ctx, "buckets")
	err = threadsclient.NewDB(ctx, id)
	require.NoError(t, err)
	ctx = common.NewThreadIDContext(ctx, id)

	t.Cleanup(func() {
		err := client.Close()
		require.NoError(t, err)
	})
	return ctx, client
}
