package client_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	ipfsfiles "github.com/ipfs/go-ipfs-files"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/apitest"
	c "github.com/textileio/textile/v2/api/bucketsd/client"
	"github.com/textileio/textile/v2/api/common"
	hc "github.com/textileio/textile/v2/api/hubd/client"
	hubpb "github.com/textileio/textile/v2/api/hubd/pb"
	bucks "github.com/textileio/textile/v2/buckets"
	"github.com/textileio/textile/v2/core"
	"github.com/textileio/textile/v2/util"
	"google.golang.org/grpc"
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

func TestClient_Create(t *testing.T) {
	ctx, client := setup(t)

	buck, err := client.Create(ctx, c.WithName("mybuck"))
	require.NoError(t, err)
	assert.NotEmpty(t, buck.Root)
	assert.NotEmpty(t, buck.Root.Key)
	assert.NotEmpty(t, buck.Root.Owner)
	assert.NotEmpty(t, buck.Root.Name)
	assert.NotEmpty(t, buck.Root.Version)
	assert.NotEmpty(t, buck.Root.Path)
	assert.NotEmpty(t, buck.Root.Metadata)
	assert.NotEmpty(t, buck.Root.CreatedAt)
	assert.NotEmpty(t, buck.Root.UpdatedAt)
	assert.NotEmpty(t, buck.Links)
	assert.NotEmpty(t, buck.Seed)

	pbuck, err := client.Create(ctx, c.WithName("myprivatebuck"), c.WithPrivate(true))
	require.NoError(t, err)
	assert.NotEmpty(t, pbuck.Root)
	assert.NotEmpty(t, pbuck.Root.Key)
	assert.NotEmpty(t, buck.Root.Owner)
	assert.NotEmpty(t, pbuck.Root.Name)
	assert.NotEmpty(t, buck.Root.Version)
	assert.NotEmpty(t, pbuck.Root.Path)
	assert.NotEmpty(t, buck.Root.Metadata)
	assert.NotEmpty(t, pbuck.Root.CreatedAt)
	assert.NotEmpty(t, pbuck.Root.UpdatedAt)
	assert.NotEmpty(t, pbuck.Links)
	assert.NotEmpty(t, pbuck.Seed)
}

func TestClient_CreateWithCid(t *testing.T) {
	ctx, client := setup(t)

	t.Run("public", func(t *testing.T) {
		createWithCid(t, ctx, client, false)
	})

	t.Run("private", func(t *testing.T) {
		createWithCid(t, ctx, client, true)
	})
}

func createWithCid(t *testing.T, ctx context.Context, client *c.Client, private bool) {
	file1, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file1.Close()
	file2, err := os.Open("testdata/file2.jpg")
	require.NoError(t, err)
	defer file2.Close()

	ipfs, err := httpapi.NewApi(apitest.GetIPFSApiAddr())
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
	assert.NotEmpty(t, root.Root.Metadata)
	assert.NotEmpty(t, root.Root.CreatedAt)
	assert.NotEmpty(t, root.Root.UpdatedAt)
}

func TestClient_Links(t *testing.T) {
	ctx, client := setup(t)

	buck, err := client.Create(ctx)
	require.NoError(t, err)

	links, err := client.Links(ctx, buck.Root.Key, "")
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

	q, err := client.PushPaths(ctx, buck.Root.Key)
	require.NoError(t, err)
	err = q.AddFile("dir1/file1.jpg", "testdata/file1.jpg")
	require.NoError(t, err)
	err = q.AddFile("dir2/file2.jpg", "testdata/file2.jpg")
	require.NoError(t, err)
	for q.Next() {
		require.NoError(t, q.Err())
	}
	q.Close()
	root := q.Current.Root

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

	ipfs, err := httpapi.NewApi(apitest.GetIPFSApiAddr())
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

func pushPath(t *testing.T, ctx context.Context, client *c.Client, private bool) {
	buck, err := client.Create(ctx, c.WithPrivate(private))
	require.NoError(t, err)

	file1, err := os.Open("testdata/file1.jpg")
	require.NoError(t, err)
	defer file1.Close()
	progress1 := make(chan int64)
	go func() {
		for p := range progress1 {
			fmt.Println(fmt.Sprintf("progress: %d", p))
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
			fmt.Println(fmt.Sprintf("progress: %d", p))
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

	// Concurrent writes should result in one being rejected due to the fast-forward-only rule
	root, err := util.NewResolvedPath(rep3.Root.Path)
	require.NoError(t, err)
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		_, _, err1 = client.PushPath(
			ctx,
			buck.Root.Key,
			"conflict",
			strings.NewReader("ready, set, go!"),
			c.WithFastForwardOnly(root),
		)
		wg.Done()
	}()
	go func() {
		_, _, err2 = client.PushPath(
			ctx,
			buck.Root.Key,
			"conflict",
			strings.NewReader("ready, set, go!"),
			c.WithFastForwardOnly(root),
		)
		wg.Done()
	}()
	wg.Wait()
	// We should have one and only one error
	assert.False(t, (err1 != nil && err2 != nil) && (err1 == nil && err2 == nil))
	if err1 != nil {
		assert.True(t, strings.Contains(err1.Error(), "update is non-fast-forward"))
	} else if err2 != nil {
		assert.True(t, strings.Contains(err2.Error(), "update is non-fast-forward"))
	}
}

func TestClient_PushPaths(t *testing.T) {
	ctx, client := setup(t)

	t.Run("public", func(t *testing.T) {
		pushPaths(t, ctx, client, false)
	})

	t.Run("private", func(t *testing.T) {
		pushPaths(t, ctx, client, true)
	})
}

func pushPaths(t *testing.T, ctx context.Context, client *c.Client, private bool) {
	buck, err := client.Create(ctx, c.WithPrivate(private))
	require.NoError(t, err)

	progress1 := make(chan int64)
	defer close(progress1)
	go func() {
		for p := range progress1 {
			fmt.Println(fmt.Sprintf("progress: %d", p))
		}
	}()

	q, err := client.PushPaths(ctx, buck.Root.Key, c.WithProgress(progress1))
	require.NoError(t, err)
	err = q.AddFile("file1.jpg", "testdata/file1.jpg")
	require.NoError(t, err)
	err = q.AddFile("path/to/file2.jpg", "testdata/file2.jpg")
	require.NoError(t, err)
	for q.Next() {
		require.NoError(t, q.Err())
		assert.NotEmpty(t, q.Current.Path)
		assert.NotEmpty(t, q.Current.Root)
	}
	q.Close()

	rep1, err := client.ListPath(ctx, buck.Root.Key, "")
	require.NoError(t, err)
	assert.Len(t, rep1.Item.Items, 3)

	// Try overwriting the path
	q2, err := client.PushPaths(ctx, buck.Root.Key)
	require.NoError(t, err)
	r := strings.NewReader("seeya!")
	err = q2.AddReader("path/to/file2.jpg", r, r.Size())
	require.NoError(t, err)
	for q2.Next() {
		require.NoError(t, q2.Err())
	}
	q2.Close()

	rep2, err := client.ListPath(ctx, buck.Root.Key, "")
	require.NoError(t, err)
	assert.Len(t, rep2.Item.Items, 3)

	// Overwrite the path again, this time replacing a file link with a dir link
	q3, err := client.PushPaths(ctx, buck.Root.Key)
	require.NoError(t, err)
	err = q3.AddFile("path/to", "testdata/file2.jpg")
	require.NoError(t, err)
	for q3.Next() {
		require.NoError(t, q3.Err())
	}
	q3.Close()

	rep3, err := client.ListPath(ctx, buck.Root.Key, "")
	require.NoError(t, err)
	assert.Len(t, rep3.Item.Items, 3)

	// Concurrent writes should result in one being rejected due to the fast-forward-only rule
	root, err := util.NewResolvedPath(rep3.Root.Path)
	require.NoError(t, err)
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		q, err := client.PushPaths(ctx, buck.Root.Key, c.WithFastForwardOnly(root))
		require.NoError(t, err)
		err = q.AddReader("conflict", strings.NewReader("ready, set, go!"), 0)
		require.NoError(t, err)
		for q.Next() {
			err1 = q.Err()
		}
		q.Close()
		wg.Done()
	}()
	go func() {
		q, err := client.PushPaths(ctx, buck.Root.Key, c.WithFastForwardOnly(root))
		require.NoError(t, err)
		err = q.AddReader("conflict", strings.NewReader("ready, set, go!"), 0)
		require.NoError(t, err)
		for q.Next() {
			err2 = q.Err()
		}
		q.Close()
		wg.Done()
	}()
	wg.Wait()
	// We should have one and only one error
	assert.False(t, (err1 != nil && err2 != nil) && (err1 == nil && err2 == nil))
	if err1 != nil {
		assert.True(t, strings.Contains(err1.Error(), "update is non-fast-forward"))
	} else if err2 != nil {
		assert.True(t, strings.Contains(err2.Error(), "update is non-fast-forward"))
	}
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
	q, err := client.PushPaths(ctx, buck.Root.Key)
	require.NoError(t, err)
	err = q.AddFile("file1.jpg", "testdata/file1.jpg")
	require.NoError(t, err)
	for q.Next() {
		require.NoError(t, q.Err())
	}
	q.Close()

	tmp, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer tmp.Close()

	progress := make(chan int64)
	go func() {
		for p := range progress {
			fmt.Println(fmt.Sprintf("progress: %d", p))
		}
	}()
	err = client.PullPath(ctx, buck.Root.Key, "file1.jpg", tmp, c.WithProgress(progress))
	require.NoError(t, err)
	info, err := tmp.Stat()
	require.NoError(t, err)
	fmt.Println(fmt.Sprintf("wrote file with size %d", info.Size()))

	note := "baps!"
	q2, err := client.PushPaths(ctx, buck.Root.Key)
	require.NoError(t, err)
	err = q2.AddReader("one/two/note.txt", strings.NewReader(note), 0)
	require.NoError(t, err)
	for q2.Next() {
		require.NoError(t, q2.Err())
	}
	q2.Close()

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

	ipfs, err := httpapi.NewApi(apitest.GetIPFSApiAddr())
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

			ipfs, err := httpapi.NewApi(apitest.GetIPFSApiAddr())
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

func TestClient_Move(t *testing.T) {
	ctx, client := setup(t)

	t.Run("public", func(t *testing.T) {
		move(t, ctx, client, false)
	})

	t.Run("private", func(t *testing.T) {
		move(t, ctx, client, true)
	})
}

func move(t *testing.T, ctx context.Context, client *c.Client, private bool) {
	buck, err := client.Create(ctx, c.WithPrivate(private))
	require.NoError(t, err)

	q, err := client.PushPaths(ctx, buck.Root.Key)
	require.NoError(t, err)
	err = q.AddFile("root.jpg", "testdata/file1.jpg")
	require.NoError(t, err)
	err = q.AddFile("a/b/c/file1.jpg", "testdata/file1.jpg")
	require.NoError(t, err)
	err = q.AddFile("a/b/c/file2.jpg", "testdata/file2.jpg")
	require.NoError(t, err)
	for q.Next() {
		require.NoError(t, q.Err())
	}
	q.Close()

	// move a dir into a new path  => a/c
	err = client.MovePath(ctx, buck.Root.Key, "a/b/c", "a/c")
	require.NoError(t, err)
	// check source files no longer exists
	li, err := client.ListPath(ctx, buck.Root.Key, "a/b/c")
	require.Error(t, err)

	// check source parent remains untouched
	li, err = client.ListPath(ctx, buck.Root.Key, "a/b")
	require.NoError(t, err)
	assert.True(t, li.Item.IsDir)
	assert.Len(t, li.Item.Items, 0)

	// move a dir into an existing path => a/b/c
	err = client.MovePath(ctx, buck.Root.Key, "a/c", "a/b")
	require.NoError(t, err)
	// check source dir no longer exists
	li, err = client.ListPath(ctx, buck.Root.Key, "a/c")
	require.Error(t, err)

	// check source parent contains all the right children
	li, err = client.ListPath(ctx, buck.Root.Key, "a")
	require.NoError(t, err)
	assert.True(t, li.Item.IsDir)
	assert.Len(t, li.Item.Items, 1)

	// check source parent contains all the right children
	li, err = client.ListPath(ctx, buck.Root.Key, "a/b/c")
	require.NoError(t, err)
	assert.True(t, li.Item.IsDir)
	assert.Len(t, li.Item.Items, 2)

	// move and rename file => a/b/c/file2.jpg + a/b/file3.jpg
	err = client.MovePath(ctx, buck.Root.Key, "a/b/c/file1.jpg", "a/b/file3.jpg")
	require.NoError(t, err)

	li, err = client.ListPath(ctx, buck.Root.Key, "a/b/file3.jpg")
	require.NoError(t, err)
	assert.False(t, li.Item.IsDir)
	assert.Equal(t, li.Item.Name, "file3.jpg")

	// move a/b/c to root => c
	err = client.MovePath(ctx, buck.Root.Key, "a/b/c", "")
	require.NoError(t, err)

	li, err = client.ListPath(ctx, buck.Root.Key, "")
	require.NoError(t, err)
	assert.True(t, li.Item.IsDir)
	assert.Len(t, li.Item.Items, 4)

	li, err = client.ListPath(ctx, buck.Root.Key, "root.jpg")
	require.NoError(t, err)

	// move root should fail
	err = client.MovePath(ctx, buck.Root.Key, "", "a")
	require.Error(t, err, "source is root directory")

	li, err = client.ListPath(ctx, buck.Root.Key, "a")
	require.NoError(t, err)
	assert.True(t, li.Item.IsDir)
	assert.Len(t, li.Item.Items, 1)

	// move non existant should fail
	err = client.MovePath(ctx, buck.Root.Key, "x", "a")
	if private {
		assert.True(t, strings.Contains(err.Error(), "could not resolve path"))
	} else {
		assert.True(t, strings.Contains(err.Error(), "no link named"))
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

	q, err := client.PushPaths(ctx, buck.Root.Key)
	require.NoError(t, err)
	err = q.AddFile("file1.jpg", "testdata/file1.jpg")
	require.NoError(t, err)
	err = q.AddFile("again/file2.jpg", "testdata/file2.jpg")
	require.NoError(t, err)
	for q.Next() {
		require.NoError(t, q.Err())
	}
	q.Close()

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

	q, err := client.PushPaths(ctx, buck.Root.Key)
	require.NoError(t, err)
	err = q.AddFile("file1.jpg", "testdata/file1.jpg")
	require.NoError(t, err)
	err = q.AddFile("again/file2.jpg", "testdata/file2.jpg")
	require.NoError(t, err)
	for q.Next() {
		require.NoError(t, q.Err())
	}
	q.Close()

	pth, err := client.RemovePath(ctx, buck.Root.Key, "again/file2.jpg")
	require.NoError(t, err)
	assert.NotEmpty(t, pth)
	_, err = client.ListPath(ctx, buck.Root.Key, "again/file2.jpg")
	require.Error(t, err)
	rep, err := client.ListPath(ctx, buck.Root.Key, "")
	require.NoError(t, err)
	assert.Len(t, rep.Item.Items, 3)

	_, err = client.RemovePath(ctx, buck.Root.Key, "again")
	require.NoError(t, err)
	_, err = client.ListPath(ctx, buck.Root.Key, "again")
	require.Error(t, err)
	rep, err = client.ListPath(ctx, buck.Root.Key, "")
	require.NoError(t, err)
	assert.Len(t, rep.Item.Items, 2)
}

func TestClient_PushPathAccessRoles(t *testing.T) {
	ctx, userctx, threadsclient, client := setupForUsers(t)

	t.Run("public", func(t *testing.T) {
		pushPathAccessRoles(t, ctx, userctx, threadsclient, client, false)
	})

	t.Run("private", func(t *testing.T) {
		pushPathAccessRoles(t, ctx, userctx, threadsclient, client, true)
	})
}

func pushPathAccessRoles(
	t *testing.T,
	ctx context.Context,
	userctx context.Context,
	threadsclient *tc.Client,
	client *c.Client,
	private bool,
) {
	buck, err := client.Create(ctx, c.WithPrivate(private))
	require.NoError(t, err)

	t.Run("non-existent path", func(t *testing.T) {
		_, pk, err := crypto.GenerateEd25519Key(rand.Reader)
		require.NoError(t, err)
		reader := thread.NewLibp2pPubKey(pk)

		err = client.PushPathAccessRoles(ctx, buck.Root.Key, "nothing/here", map[string]bucks.Role{
			reader.String(): bucks.Reader,
		})
		require.Error(t, err)
	})

	t.Run("existent path", func(t *testing.T) {
		user1, user1ctx := newUser(t, userctx, threadsclient)

		q, err := client.PushPaths(ctx, buck.Root.Key)
		require.NoError(t, err)
		err = q.AddFile("file", "testdata/file1.jpg")
		require.NoError(t, err)
		for q.Next() {
			require.NoError(t, q.Err())
		}
		q.Close()

		// Check initial access (none)
		check := accessCheck{
			Key:  buck.Root.Key,
			Path: "file",
		}
		check.Read = !private // public buckets readable by all
		checkAccess(t, user1ctx, client, check)

		// Grant reader
		err = client.PushPathAccessRoles(ctx, buck.Root.Key, "file", map[string]bucks.Role{
			user1.GetPublic().String(): bucks.Reader,
		})
		require.NoError(t, err)
		check.Read = true
		checkAccess(t, user1ctx, client, check)

		// Grant writer
		err = client.PushPathAccessRoles(ctx, buck.Root.Key, "file", map[string]bucks.Role{
			user1.GetPublic().String(): bucks.Writer,
		})
		require.NoError(t, err)
		check.Write = true
		checkAccess(t, user1ctx, client, check)

		// Grant admin
		err = client.PushPathAccessRoles(ctx, buck.Root.Key, "file", map[string]bucks.Role{
			user1.GetPublic().String(): bucks.Admin,
		})
		require.NoError(t, err)
		check.Admin = true
		checkAccess(t, user1ctx, client, check)

		// Ungrant
		err = client.PushPathAccessRoles(ctx, buck.Root.Key, "file", map[string]bucks.Role{
			user1.GetPublic().String(): bucks.None,
		})
		require.NoError(t, err)
		check.Read = !private
		check.Write = false
		check.Admin = false
		checkAccess(t, user1ctx, client, check)
	})

	t.Run("overlapping paths", func(t *testing.T) {
		user1, user1ctx := newUser(t, userctx, threadsclient)
		user2, user2ctx := newUser(t, userctx, threadsclient)

		q, err := client.PushPaths(ctx, buck.Root.Key)
		require.NoError(t, err)
		err = q.AddFile("a/b/f1", "testdata/file1.jpg")
		require.NoError(t, err)
		for q.Next() {
			require.NoError(t, q.Err())
		}
		q.Close()

		q2, err := client.PushPaths(ctx, buck.Root.Key)
		require.NoError(t, err)
		err = q2.AddFile("a/f2", "testdata/file2.jpg")
		require.NoError(t, err)
		for q2.Next() {
			require.NoError(t, q2.Err())
		}
		q2.Close()

		// Grant nested tree
		err = client.PushPathAccessRoles(ctx, buck.Root.Key, "a/b", map[string]bucks.Role{
			user1.GetPublic().String(): bucks.Writer,
		})
		require.NoError(t, err)
		checkAccess(t, user1ctx, client, accessCheck{
			Key:   buck.Root.Key,
			Path:  "a/b/f1",
			Read:  true,
			Write: true,
		})

		// Grant parent of nested tree
		err = client.PushPathAccessRoles(ctx, buck.Root.Key, "a", map[string]bucks.Role{
			user2.GetPublic().String(): bucks.Writer,
		})
		require.NoError(t, err)
		checkAccess(t, user2ctx, client, accessCheck{
			Key:   buck.Root.Key,
			Path:  "a/b/f1",
			Read:  true,
			Write: true,
		})

		// Re-check nested access for user1
		checkAccess(t, user1ctx, client, accessCheck{
			Key:   buck.Root.Key,
			Path:  "a/b/f1",
			Read:  true,
			Write: true,
		})

		// Grant root
		err = client.PushPathAccessRoles(ctx, buck.Root.Key, "", map[string]bucks.Role{
			user2.GetPublic().String(): bucks.Reader,
		})
		require.NoError(t, err)
		checkAccess(t, user2ctx, client, accessCheck{
			Key:   buck.Root.Key,
			Path:  "a/f2",
			Read:  true,
			Write: true,
		})

		// Re-check nested access for user1
		checkAccess(t, user1ctx, client, accessCheck{
			Key:   buck.Root.Key,
			Path:  "a/b/f1",
			Read:  true,
			Write: true,
		})
	})

	t.Run("moving path", func(t *testing.T) {
		user1, user1ctx := newUser(t, userctx, threadsclient)

		q, err := client.PushPaths(ctx, buck.Root.Key)
		require.NoError(t, err)
		err = q.AddFile("moving/file", "testdata/file1.jpg")
		require.NoError(t, err)
		for q.Next() {
			require.NoError(t, q.Err())
		}
		q.Close()

		// Check initial access (none)
		check := accessCheck{
			Key:  buck.Root.Key,
			Path: "moving/file",
		}
		check.Read = !private // public buckets readable by all
		checkAccess(t, user1ctx, client, check)

		// Grant reader
		err = client.PushPathAccessRoles(ctx, buck.Root.Key, "moving/file", map[string]bucks.Role{
			user1.GetPublic().String(): bucks.Writer,
		})
		require.NoError(t, err)
		check.Write = true
		check.Read = true
		checkAccess(t, user1ctx, client, check)

		// move the shared file to a new path
		err = client.MovePath(ctx, buck.Root.Key, "moving/file", "moving/file2")
		require.NoError(t, err)
		q, err = client.PushPaths(ctx, buck.Root.Key)
		require.NoError(t, err)
		err = q.AddFile("moving/file", "testdata/file1.jpg")
		require.NoError(t, err)
		for q.Next() {
			require.NoError(t, q.Err())
		}
		q.Close()

		// Permissions reset with move
		checkAccess(t, user1ctx, client, accessCheck{
			Key:   buck.Root.Key,
			Path:  "moving/file2",
			Admin: false,
			Read:  !private,
			Write: false,
		})

		// Grant admin at root
		err = client.PushPathAccessRoles(ctx, buck.Root.Key, "moving", map[string]bucks.Role{
			user1.GetPublic().String(): bucks.Admin,
		})
		require.NoError(t, err)

		// now user has access to new file again
		checkAccess(t, user1ctx, client, accessCheck{
			Key:   buck.Root.Key,
			Path:  "moving/file2",
			Admin: true,
			Read:  true,
			Write: true,
		})

		// Move file again
		err = client.MovePath(ctx, buck.Root.Key, "moving/file2", "moving/file3")
		require.NoError(t, err)

		// User still has access to shared file after move
		checkAccess(t, user1ctx, client, accessCheck{
			Key:   buck.Root.Key,
			Path:  "moving/file3",
			Admin: true,
			Read:  true,
			Write: true,
		})
	})
}

type accessCheck struct {
	Key  string
	Path string

	Read  bool
	Write bool
	Admin bool
}

func checkAccess(t *testing.T, ctx context.Context, client *c.Client, check accessCheck) {
	// Check read access
	res, err := client.ListPath(ctx, check.Key, check.Path)
	if check.Read {
		require.NoError(t, err)
		assert.NotEmpty(t, res.Item)
	} else {
		require.Error(t, err)
	}

	// Check write access
	tmp, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer tmp.Close()
	_, err = io.CopyN(tmp, rand.Reader, 1024)
	require.NoError(t, err)
	q, err := client.PushPaths(ctx, check.Key)
	require.NoError(t, err)
	err = q.AddReader(check.Path, tmp, 0)
	require.NoError(t, err)
	for q.Next() {
		if check.Write {
			require.NoError(t, q.Err())
		} else {
			require.Error(t, q.Err())
		}
	}
	q.Close()

	// Check admin access (role editing)
	_, pk, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	err = client.PushPathAccessRoles(ctx, check.Key, check.Path, map[string]bucks.Role{
		thread.NewLibp2pPubKey(pk).String(): bucks.Reader,
	})
	if check.Admin {
		require.NoError(t, err)
	} else {
		require.Error(t, err)
	}
}

func TestClient_PullPathAccessRoles(t *testing.T) {
	ctx, client := setup(t)

	buck, err := client.Create(ctx)
	require.NoError(t, err)
	q, err := client.PushPaths(ctx, buck.Root.Key)
	require.NoError(t, err)
	err = q.AddFile("file1.jpg", "testdata/file1.jpg")
	require.NoError(t, err)
	for q.Next() {
		require.NoError(t, q.Err())
	}
	q.Close()

	roles, err := client.PullPathAccessRoles(ctx, buck.Root.Key, "file1.jpg")
	require.NoError(t, err)
	assert.Len(t, roles, 0)

	_, pk, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	reader := thread.NewLibp2pPubKey(pk)
	roles = map[string]bucks.Role{
		reader.String(): bucks.Reader,
	}
	err = client.PushPathAccessRoles(ctx, buck.Root.Key, "file1.jpg", roles)
	require.NoError(t, err)

	roles, err = client.PullPathAccessRoles(ctx, buck.Root.Key, "file1.jpg")
	require.NoError(t, err)
	assert.Len(t, roles, 1)
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
	bconf := apitest.DefaultBillingConfig(t)
	apitest.MakeBillingWithConfig(t, bconf)

	conf := apitest.DefaultTextileConfig(t)
	billingApi, err := tutil.TCPAddrFromMultiAddr(bconf.ListenAddr)
	require.NoError(t, err)
	conf.AddrBillingAPI = billingApi
	ctx, _, _, client := setupWithConf(t, conf)
	return ctx, client
}

func setupForUsers(t *testing.T) (context.Context, context.Context, *tc.Client, *c.Client) {
	conf := apitest.DefaultTextileConfig(t)
	ctx, hubclient, threadsclient, client := setupWithConf(t, conf)

	res, err := hubclient.CreateKey(ctx, hubpb.KeyType_KEY_TYPE_USER, true)
	require.NoError(t, err)
	userctx := common.NewAPIKeyContext(context.Background(), res.KeyInfo.Key)
	userctx, err = common.CreateAPISigContext(userctx, time.Now().Add(time.Hour), res.KeyInfo.Secret)
	require.NoError(t, err)

	id, ok := common.ThreadIDFromContext(ctx)
	require.True(t, ok)
	userctx = common.NewThreadIDContext(userctx, id)

	return ctx, userctx, threadsclient, client
}

func setupWithConf(t *testing.T, conf core.Config) (context.Context, *hc.Client, *tc.Client, *c.Client) {
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
	return ctx, hubclient, threadsclient, client
}

func newUser(t *testing.T, ctx context.Context, threadsclient *tc.Client) (thread.Identity, context.Context) {
	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	identity := thread.NewLibp2pIdentity(sk)
	tok, err := threadsclient.GetToken(ctx, identity)
	require.NoError(t, err)
	return identity, thread.NewTokenContext(ctx, tok)
}
