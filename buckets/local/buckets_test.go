package local_test

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"testing"

	ipfsfiles "github.com/ipfs/go-ipfs-files"
	httpapi "github.com/ipfs/go-ipfs-http-client"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/apitest"
	. "github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/textile/util"
)

func TestBuckets_NewBucket(t *testing.T) {
	t.Parallel()
	buckets := setup(t)

	t.Run("new bucket", func(t *testing.T) {
		buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
		require.Nil(t, err)
		assert.NotEmpty(t, buck)

		info, err := buck.Info(context.Background())
		require.Nil(t, err)
		assert.NotEmpty(t, info)
	})

	t.Run("new named bucket", func(t *testing.T) {
		buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets), WithName("bucky"))
		require.Nil(t, err)
		assert.NotEmpty(t, buck)

		info, err := buck.Info(context.Background())
		require.Nil(t, err)
		assert.Equal(t, "bucky", info.Name)
	})

	t.Run("new private bucket", func(t *testing.T) {
		buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets), WithPrivate(true))
		require.Nil(t, err)
		assert.NotEmpty(t, buck)

		info, err := buck.Info(context.Background())
		require.Nil(t, err)
		assert.NotEmpty(t, info)
	})

	t.Run("new bootstrapped bucket", func(t *testing.T) {
		pth := createIpfsFolder(t)
		events := make(chan PathEvent)
		defer close(events)
		ec := &eventCollector{}
		go ec.collect(events)
		buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets), WithCid(pth.Cid()), WithExistingPathEvents(events))
		require.Nil(t, err)
		assert.NotEmpty(t, buck)
		ec.check(t, 2, 0)

		info, err := buck.Info(context.Background())
		require.Nil(t, err)
		assert.NotEmpty(t, info)

		items, err := buck.ListRemotePath(context.Background(), "")
		require.Nil(t, err)
		assert.Len(t, items, 3)

		bp, err := buck.Path()
		require.Nil(t, err)
		_, err = os.Stat(filepath.Join(bp, "file1.txt"))
		require.Nil(t, err)
		_, err = os.Stat(filepath.Join(bp, "folder1", "file2.txt"))
		require.Nil(t, err)
	})

	t.Run("new bucket from existing", func(t *testing.T) {
		buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
		require.Nil(t, err)
		assert.NotEmpty(t, buck)

		addRandomFile(t, buck, "file1", 256)
		addRandomFile(t, buck, "folder/file2", 256)
		_, err = buck.PushLocal(context.Background())
		require.Nil(t, err)

		conf2 := Config{Path: newDir(t)}
		conf2.Key = buck.Key()
		conf2.Thread, err = buck.Thread()
		require.Nil(t, err)
		buck2, err := buckets.NewBucket(context.Background(), conf2)
		require.Nil(t, err)
		require.NotEmpty(t, buck2)

		items, err := buck2.ListRemotePath(context.Background(), "")
		require.Nil(t, err)
		assert.Len(t, items, 3)

		bp, err := buck.Path()
		require.Nil(t, err)
		_, err = os.Stat(filepath.Join(bp, "file1"))
		require.Nil(t, err)
		_, err = os.Stat(filepath.Join(bp, "folder", "file2"))
		require.Nil(t, err)
	})
}

func TestBuckets_GetLocalBucket(t *testing.T) {
	t.Parallel()
	buckets := setup(t)

	conf := getConf(t, buckets)
	_, err := buckets.NewBucket(context.Background(), conf)
	require.Nil(t, err)

	buck, err := buckets.GetLocalBucket(context.Background(), conf.Path)
	require.Nil(t, err)
	assert.NotEmpty(t, buck)
}

func TestBuckets_RemoteBuckets(t *testing.T) {
	t.Parallel()
	buckets := setup(t)

	list, err := buckets.RemoteBuckets(context.Background())
	require.Nil(t, err)
	assert.Len(t, list, 0)

	_, err = buckets.NewBucket(context.Background(), getConf(t, buckets))
	assert.Nil(t, err)

	list, err = buckets.RemoteBuckets(context.Background())
	require.Nil(t, err)
	assert.Len(t, list, 1)
}

func TestBuckets_NewConfigFromCmd(t *testing.T) {
	t.Parallel()
	buckets := setup(t)

	t.Run("no flags", func(t *testing.T) {
		c := initCmd(t, buckets, "", thread.Undef, false, false)
		err := c.Execute()
		require.Nil(t, err)
	})

	t.Run("with flags and no values", func(t *testing.T) {
		c := initCmd(t, buckets, "", thread.Undef, true, false)
		err := c.Execute()
		require.Nil(t, err)
	})

	t.Run("with flags and default values", func(t *testing.T) {
		key := "mykey"
		tid := thread.NewIDV1(thread.Raw, 32)
		c := initCmd(t, buckets, key, tid, true, true)
		err := c.Execute()
		require.Nil(t, err)
	})

	t.Run("with flags and set values", func(t *testing.T) {
		key := "mykey"
		tid := thread.NewIDV1(thread.Raw, 32)
		c := initCmd(t, buckets, key, tid, true, false)
		err := c.PersistentFlags().Set("key", key)
		require.Nil(t, err)
		err = c.PersistentFlags().Set("thread", tid.String())
		require.Nil(t, err)
		err = c.Execute()
		require.Nil(t, err)
	})

	t.Run("no flags and env values", func(t *testing.T) {
		key := "mykey"
		tid := thread.NewIDV1(thread.Raw, 32)
		c := initCmd(t, buckets, key, tid, false, false)
		err := os.Setenv("BUCK_KEY", key)
		require.Nil(t, err)
		err = os.Setenv("BUCK_THREAD", tid.String())
		require.Nil(t, err)
		err = c.Execute()
		require.Nil(t, err)
	})

	t.Run("with flags and env values", func(t *testing.T) {
		key := "mykey"
		tid := thread.NewIDV1(thread.Raw, 32)
		c := initCmd(t, buckets, key, tid, true, false)
		err := os.Setenv("BUCK_KEY", key)
		require.Nil(t, err)
		err = os.Setenv("BUCK_THREAD", tid.String())
		require.Nil(t, err)
		err = c.Execute()
		require.Nil(t, err)
	})

	t.Run("with key and no thread", func(t *testing.T) {
		dir := newDir(t)
		c := &cobra.Command{
			Use: "init",
			Run: func(c *cobra.Command, args []string) {
				_, err := buckets.NewConfigFromCmd(c, dir)
				require.NotNil(t, err)
				assert.Equal(t, ErrThreadRequired, err)
			},
		}
		err := os.Setenv("BUCK_KEY", "mykey")
		require.Nil(t, err)
		err = os.Setenv("BUCK_THREAD", "")
		require.Nil(t, err)
		err = c.Execute()
		require.Nil(t, err)
	})
}

func initCmd(t *testing.T, buckets *Buckets, key string, tid thread.ID, addFlags, setDefaults bool) *cobra.Command {
	dir := newDir(t)
	c := &cobra.Command{
		Use: "init",
		Run: func(c *cobra.Command, args []string) {
			conf, err := buckets.NewConfigFromCmd(c, dir)
			require.Nil(t, err)
			assert.Equal(t, dir, conf.Path)
			assert.Equal(t, key, conf.Key)
			if tid.Defined() {
				assert.Equal(t, tid, conf.Thread)
			} else {
				assert.Equal(t, thread.Undef, conf.Thread)
			}
		},
	}
	var dkey, dtid string
	if setDefaults {
		dkey = key
		dtid = tid.String()
	}
	if addFlags {
		c.PersistentFlags().String("key", dkey, "")
		c.PersistentFlags().String("thread", dtid, "")
	}
	return c
}

func setup(t *testing.T) *Buckets {
	conf := apitest.DefaultTextileConfig(t)
	conf.Hub = false
	apitest.MakeTextileWithConfig(t, conf)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.Nil(t, err)

	clients := cmd.NewClients(target, false)
	t.Cleanup(func() {
		clients.Close()
	})

	return NewBuckets(clients, DefaultConfConfig())
}

func getConf(t *testing.T, bucks *Buckets) Config {
	id := thread.NewIDV1(thread.Raw, 32)
	err := bucks.Clients().Threads.NewDB(context.Background(), id)
	require.Nil(t, err)

	return Config{
		Path:   newDir(t),
		Thread: id,
	}
}

func newDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}

func createIpfsFolder(t *testing.T) (pth path.Resolved) {
	ipfs, err := httpapi.NewApi(util.MustParseAddr("/ip4/127.0.0.1/tcp/5001"))
	require.NoError(t, err)
	pth, err = ipfs.Unixfs().Add(
		context.Background(),
		ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
			"file1.txt": ipfsfiles.NewBytesFile(util.GenerateRandomBytes(1024)),
			"folder1": ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
				"file2.txt": ipfsfiles.NewBytesFile(util.GenerateRandomBytes(512)),
			}),
		}),
	)
	require.Nil(t, err)
	return pth
}

type eventCollector struct {
	pathStarted   bool
	pathCompleted bool
	fileStarts    int
	fileCompletes int
	fileRemoves   int
	sync.Mutex
}

func (c *eventCollector) collect(events chan PathEvent) {
	for e := range events {
		c.Lock()
		switch e.Type {
		case PathStart:
			c.pathStarted = true
		case PathComplete:
			c.pathCompleted = true
		case FileStart:
			c.fileStarts++
		case FileComplete:
			c.fileCompletes++
		case FileRemoved:
			c.fileRemoves++
		}
		c.Unlock()
	}
	return
}

func (c *eventCollector) check(t *testing.T, numFilesAdded, numFilesRemoved int) {
	c.Lock()
	defer c.Unlock()
	if numFilesAdded > 0 {
		assert.True(t, c.pathStarted)
		assert.True(t, c.pathCompleted)
	}
	assert.Equal(t, numFilesAdded, c.fileStarts)
	assert.Equal(t, numFilesAdded, c.fileCompletes)
	assert.Equal(t, numFilesRemoved, c.fileRemoves)
}
