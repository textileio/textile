package local_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	du "github.com/ipfs/go-merkledag/dagutils"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/dcrypto"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/apitest"
	bucks "github.com/textileio/textile/v2/buckets"
	. "github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
)

func TestBucket(t *testing.T) {
	buckets := setup(t)
	conf := getConf(t, buckets)
	buck, err := buckets.NewBucket(context.Background(), conf, WithName("bucky"))
	require.NoError(t, err)

	t.Run("Key", func(t *testing.T) {
		assert.NotEmpty(t, buck.Key())
	})

	t.Run("Thread", func(t *testing.T) {
		tid, err := buck.Thread()
		require.NoError(t, err)
		assert.Equal(t, conf.Thread, tid)
	})

	t.Run("Path", func(t *testing.T) {
		bp, err := buck.Path()
		require.NoError(t, err)
		assert.Equal(t, conf.Path, bp)
	})

	t.Run("LocalSize", func(t *testing.T) {
		addRandomFile(t, buck, "file", 256)
		addRandomFile(t, buck, "dir/file", 256)
		size, err := buck.LocalSize()
		require.NoError(t, err)
		assert.Equal(t, 512+32, int(size)) // Account for seed size
	})

	t.Run("Info", func(t *testing.T) {
		info, err := buck.Info(context.Background())
		require.NoError(t, err)
		assert.NotEmpty(t, info.Key)
		assert.Equal(t, info.Name, "bucky")
		assert.NotEmpty(t, info.Path)
		assert.Equal(t, info.Thread, conf.Thread)
		assert.NotEmpty(t, info.CreatedAt)
		assert.NotEmpty(t, info.UpdatedAt)
	})

	t.Run("Roots", func(t *testing.T) {
		roots, err := buck.Roots(context.Background())
		require.NoError(t, err)
		assert.True(t, roots.Local.Defined())
		assert.True(t, roots.Remote.Defined())
	})

	t.Run("RemoteLinks", func(t *testing.T) {
		links, err := buck.RemoteLinks(context.Background(), "")
		require.NoError(t, err)
		assert.NotEmpty(t, links.URL)
		assert.NotEmpty(t, links.IPNS)
	})

	t.Run("DBInfo", func(t *testing.T) {
		dbinfo, cc, err := buck.DBInfo(context.Background())
		require.NoError(t, err)
		assert.True(t, dbinfo.Key.Defined())
		assert.NotEmpty(t, dbinfo.Addrs)
		assert.NotEmpty(t, cc.Name)
		assert.NotEmpty(t, cc.Schema)
		assert.NotEmpty(t, cc.WriteValidator)
		assert.NotEmpty(t, cc.ReadFilter)
		assert.NotEmpty(t, cc.Indexes)
	})

	t.Run("Destroy", func(t *testing.T) {
		err = buck.Destroy(context.Background())
		require.NoError(t, err)
		// Ensure the local bucket was removed
		_, err = buckets.GetLocalBucket(context.Background(), conf)
		require.Error(t, err)
		// Ensure the remote bucket was removed
		list, err := buckets.RemoteBuckets(context.Background(), thread.Undef)
		require.NoError(t, err)
		assert.Len(t, list, 0)
	})

	t.Run("EncryptLocalPath", func(t *testing.T) {
		fpth := addRandomFile(t, buck, "plaintext", 1024)
		key, err := dcrypto.NewKey()
		require.NoError(t, err)
		var buf bytes.Buffer
		err = buck.EncryptLocalPath(fpth, key, &buf)
		require.NoError(t, err)
	})

	t.Run("DecryptLocalPath", func(t *testing.T) {
		fpth := addRandomFile(t, buck, "plaintext", 1024)
		cipher := filepath.Join(filepath.Dir(fpth), "ciphertext")
		f, err := os.Create(cipher)
		require.NoError(t, err)
		key, err := dcrypto.NewKey()
		require.NoError(t, err)
		err = buck.EncryptLocalPath(fpth, key, f)
		require.NoError(t, err)
		f.Close()
		var buf bytes.Buffer
		err = buck.DecryptLocalPath(cipher, []byte("badpass"), &buf)
		require.Error(t, err)
		err = buck.DecryptLocalPath(cipher, key, &buf)
		require.NoError(t, err)
		assert.Equal(t, 1024, buf.Len())
	})

	t.Run("EncryptLocalPathWithPassword", func(t *testing.T) {
		fpth := addRandomFile(t, buck, "plaintext", 1024)
		var buf bytes.Buffer
		err = buck.EncryptLocalPathWithPassword(fpth, "shhhhh!", &buf)
		require.NoError(t, err)
	})

	t.Run("DecryptLocalPathWithPassword", func(t *testing.T) {
		fpth := addRandomFile(t, buck, "plaintext", 1024)
		cipher := filepath.Join(filepath.Dir(fpth), "ciphertext")
		f, err := os.Create(cipher)
		require.NoError(t, err)
		err = buck.EncryptLocalPathWithPassword(fpth, "shhhhh!", f)
		require.NoError(t, err)
		f.Close()
		var buf bytes.Buffer
		err = buck.DecryptLocalPathWithPassword(cipher, "badpass", &buf)
		require.Error(t, err)
		err = buck.DecryptLocalPathWithPassword(cipher, "shhhhh!", &buf)
		require.NoError(t, err)
		assert.Equal(t, 1024, buf.Len())
	})
}

func TestBucket_PushLocal(t *testing.T) {
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.NoError(t, err)

	_, err = buck.PushLocal(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUpToDate))

	addRandomFile(t, buck, "file", 1024)

	roots, err := buck.PushLocal(context.Background())
	require.NoError(t, err)
	assert.True(t, roots.Local.Defined())
	assert.True(t, roots.Remote.Defined())

	_, err = buck.PushLocal(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUpToDate))
}

func TestBucket_PullRemote(t *testing.T) {
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.NoError(t, err)

	_, err = buck.PullRemote(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUpToDate))

	fpth := addRandomFile(t, buck, "dir/file", 1024)
	_, err = buck.PushLocal(context.Background())
	require.NoError(t, err)

	// Delete the file locally
	err = os.RemoveAll(fpth)
	require.NoError(t, err)

	roots, err := buck.PullRemote(context.Background())
	require.NoError(t, err)
	assert.True(t, roots.Local.Defined())
	assert.True(t, roots.Remote.Defined())

	// The local removal should be maintained
	diff, err := buck.DiffLocal()
	require.NoError(t, err)
	assert.Len(t, diff, 1)
	if len(diff) > 0 {
		assert.Equal(t, diff[0].Type, du.Remove)
	}

	// Pulling hard should reset the local to the exact state of the remote
	_, err = buck.PullRemote(context.Background(), WithHard(true))
	require.NoError(t, err)

	_, err = buck.PullRemote(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUpToDate))

	// Create another bucket to test an edge case where,
	// 1. A file is removed from the remote via local bucket 1
	// 2. The same file is modified in local bucket 2
	// 3. Local bucket 2 pulls hard on the remote
	// 4. The bucket should only report 1 removal event, not one from the remote
	//    and one from the removal of the local change.
	conf2 := Config{Path: newDir(t)}
	conf2.Key = buck.Key()
	conf2.Thread, err = buck.Thread()
	require.NoError(t, err)
	_, err = buckets.NewBucket(context.Background(), conf2)
	require.NoError(t, err)

	// Use a bucket from a nested folder to test if the relative path matching is working
	buck2, err := buckets.GetLocalBucket(context.Background(), Config{Path: filepath.Join(conf2.Path, "dir")})
	require.NoError(t, err)

	// Delete the file locally from the first bucket
	err = os.RemoveAll(fpth)
	require.NoError(t, err)
	_, err = buck.PushLocal(context.Background())
	require.NoError(t, err)

	// Stage a modification to the same file in the second bucket
	addRandomFile(t, buck2, "dir/file", 1024)

	// Pull the remote, the local change should be cleared
	// with only one remove event
	events := make(chan Event)
	defer close(events)
	ec := &eventCollector{}
	go ec.collect(events)
	_, err = buck2.PullRemote(context.Background(), WithHard(true), WithEvents(events))
	require.NoError(t, err)
	ec.check(t, 0, 1)
}

func TestBucket_AddRemoteCid(t *testing.T) {
	buckets := setup(t)
	conf := getConf(t, buckets)
	buck, err := buckets.NewBucket(context.Background(), conf)
	require.NoError(t, err)

	pth := createIpfsFolder(t)
	err = buck.AddRemoteCid(context.Background(), pth.Cid(), conf.Path)
	require.NoError(t, err)

	bp, err := buck.Path()
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(bp, "file1.txt"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(bp, "folder1", "file2.txt"))
	require.NoError(t, err)
}

func TestBucket_RemotePaths(t *testing.T) {
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.NoError(t, err)

	t.Run("ListRemotePath", func(t *testing.T) {
		items, err := buck.ListRemotePath(context.Background(), "")
		require.NoError(t, err)
		assert.Len(t, items, 1)

		addRandomFile(t, buck, "dir/file", 1024)
		_, err = buck.PushLocal(context.Background())
		require.NoError(t, err)

		items, err = buck.ListRemotePath(context.Background(), "")
		require.NoError(t, err)
		assert.Len(t, items, 2)

		items, err = buck.ListRemotePath(context.Background(), "dir")
		require.NoError(t, err)
		assert.Len(t, items, 1)
	})

	t.Run("CatRemotePath", func(t *testing.T) {
		var buf bytes.Buffer
		err = buck.CatRemotePath(context.Background(), "dir/file", &buf)
		require.NoError(t, err)
		assert.Equal(t, 1024, buf.Len())
	})

	t.Run("DecryptRemotePath", func(t *testing.T) {
		fpth := addRandomFile(t, buck, "plaintext", 1024)
		cipher := filepath.Join(filepath.Dir(fpth), "ciphertext")
		f, err := os.Create(cipher)
		require.NoError(t, err)
		defer f.Close()
		key, err := dcrypto.NewKey()
		require.NoError(t, err)
		err = buck.EncryptLocalPath(fpth, key, f)
		require.NoError(t, err)
		err = os.RemoveAll(fpth)
		require.NoError(t, err)
		_, err = buck.PushLocal(context.Background())
		require.NoError(t, err)

		var buf2 bytes.Buffer
		err = buck.DecryptRemotePath(context.Background(), "ciphertext", []byte("badpass"), &buf2)
		require.Error(t, err)
		err = buck.DecryptRemotePath(context.Background(), "ciphertext", key, &buf2)
		require.NoError(t, err)
		assert.Equal(t, 1024, buf2.Len())
	})

	t.Run("DecryptRemotePathWithPassword", func(t *testing.T) {
		fpth := addRandomFile(t, buck, "plaintext", 1024)
		cipher := filepath.Join(filepath.Dir(fpth), "ciphertext")
		f, err := os.Create(cipher)
		require.NoError(t, err)
		defer f.Close()
		err = buck.EncryptLocalPathWithPassword(fpth, "shhhhh!", f)
		require.NoError(t, err)
		err = os.RemoveAll(fpth)
		require.NoError(t, err)
		_, err = buck.PushLocal(context.Background())
		require.NoError(t, err)

		var buf2 bytes.Buffer
		err = buck.DecryptRemotePathWithPassword(context.Background(), "ciphertext", "badpass", &buf2)
		require.Error(t, err)
		err = buck.DecryptRemotePathWithPassword(context.Background(), "ciphertext", "shhhhh!", &buf2)
		require.NoError(t, err)
		assert.Equal(t, 1024, buf2.Len())
	})
}

func TestBucket_DiffLocal(t *testing.T) {
	buckets := setup(t)
	conf := getConf(t, buckets)
	buck, err := buckets.NewBucket(context.Background(), conf)
	require.NoError(t, err)

	addRandomFile(t, buck, "file1", 256)
	fpth := addRandomFile(t, buck, "folder/file2", 256)
	diff, err := buck.DiffLocal()
	require.NoError(t, err)
	assert.Len(t, diff, 2)
	assert.Equal(t, diff[0].Type, du.Add)

	// Test diff from a nested folder
	buck2, err := buckets.GetLocalBucket(context.Background(), Config{Path: filepath.Join(conf.Path, "folder")})
	require.NoError(t, err)
	diff, err = buck2.DiffLocal()
	require.NoError(t, err)
	assert.Len(t, diff, 2)
	assert.Equal(t, diff[0].Type, du.Add)

	_, err = buck.PushLocal(context.Background())
	require.NoError(t, err)

	diff, err = buck.DiffLocal()
	require.NoError(t, err)
	assert.Len(t, diff, 0)

	addRandomFile(t, buck, "file1", 256)
	diff, err = buck.DiffLocal()
	require.NoError(t, err)
	assert.Len(t, diff, 1)
	assert.Equal(t, diff[0].Type, du.Mod)

	_, err = buck.PushLocal(context.Background())
	require.NoError(t, err)

	diff, err = buck.DiffLocal()
	require.NoError(t, err)
	assert.Len(t, diff, 0)

	err = os.RemoveAll(fpth)
	require.NoError(t, err)
	diff, err = buck.DiffLocal()
	require.NoError(t, err)
	assert.Len(t, diff, 1)
	assert.Equal(t, diff[0].Type, du.Remove)

	_, err = buck.PushLocal(context.Background())
	require.NoError(t, err)

	diff, err = buck.DiffLocal()
	require.NoError(t, err)
	assert.Len(t, diff, 0)
}

func TestBucket_Watch(t *testing.T) {
	tconf := apitest.DefaultTextileConfig(t)
	tconf.Hub = false
	stopTextile1 := apitest.MakeTextileWithConfig(t, tconf, apitest.WithoutAutoShutdown())
	target, err := tutil.TCPAddrFromMultiAddr(tconf.AddrAPI)
	require.NoError(t, err)
	clients := cmd.NewClients(target, false, "")
	buckets1 := NewBuckets(clients, DefaultConfConfig())
	defer clients.Close()

	conf := getConf(t, buckets1)
	buck1, err := buckets1.NewBucket(context.Background(), conf)
	require.NoError(t, err)

	events := make(chan Event)
	defer close(events)
	ec := &eventCollector{}
	go ec.collect(events)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var onlineStateCount, offlineStateCount int
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		state, err := buck1.Watch(ctx, WithWatchEvents(events), WithOffline(true))
		require.NoError(t, err)
		for s := range state {
			fmt.Println(fmt.Sprintf("received watch state: %s", s.State))
			switch s.State {
			case cmd.Online:
				onlineStateCount++
			case cmd.Offline:
				offlineStateCount++
			}
		}
	}()

	// Add a file to the first bucket
	addRandomFile(t, buck1, "file1", 512)

	// Wait a sec while the watcher kicks off a watch cycle
	time.Sleep(time.Second * 5)
	// Watch should have handled the diff
	_, err = buck1.PushLocal(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUpToDate))

	// Create the same bucket from a different daemon
	buckets2 := setup(t)
	tid, err := buck1.Thread()
	require.NoError(t, err)
	dbinfo, err := buckets1.Clients().Threads.GetDBInfo(context.Background(), tid)
	require.NoError(t, err)
	cc, err := buckets1.Clients().Threads.GetCollectionInfo(context.Background(), tid, bucks.CollectionName)
	require.NoError(t, err)

	err = buckets2.Clients().Threads.NewDBFromAddr(
		context.Background(),
		dbinfo.Addrs[0],
		dbinfo.Key,
		db.WithNewManagedName(dbinfo.Name),
		db.WithNewManagedCollections(cc),
		db.WithNewManagedBackfillBlock(true))
	require.NoError(t, err)

	// Wait a sec while the bucket is backfilled
	time.Sleep(time.Second * 5)
	buck2, err := buckets2.NewBucket(context.Background(), Config{
		Path:   newDir(t),
		Key:    buck1.Key(),
		Thread: tid,
	})
	require.NoError(t, err)

	// Add another file to the second bucket
	addRandomFile(t, buck2, "file2", 512)
	_, err = buck2.PushLocal(context.Background())
	require.NoError(t, err)

	// Wait a sec while the remote event is handled
	time.Sleep(time.Second * 5)
	// Watch should have handled the remote diff
	_, err = buck1.PullRemote(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUpToDate))

	bp, err := buck1.Path()
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(bp, "file2"))
	require.NoError(t, err)

	ec.check(t, 2, 0)

	// Stop and restart the first bucket's remote
	stopTextile1()
	stopTextile1Again := apitest.MakeTextileWithConfig(t, tconf, apitest.WithoutAutoShutdown())
	time.Sleep(time.Second * 10) // Wait a sec for good measure
	stopTextile1Again()
	cancel() // Stop watching
	wg.Wait()
	assert.Equal(t, 2, onlineStateCount)    // 1 for the initial start, 1 for the restart
	assert.Greater(t, offlineStateCount, 0) // At least one, but could be more as watch retries
}

func TestBucket_AccessRoles(t *testing.T) {
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.NoError(t, err)

	addRandomFile(t, buck, "file", 1024)

	_, err = buck.PushLocal(context.Background())
	require.NoError(t, err)

	roles, err := buck.PullPathAccessRoles(context.Background(), "file")
	require.NoError(t, err)
	assert.Len(t, roles, 0)

	_, rpk, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	reader := thread.NewLibp2pPubKey(rpk).String()
	_, wpk, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)
	writer := thread.NewLibp2pPubKey(wpk).String()
	all, err := buck.PushPathAccessRoles(context.Background(), "file", map[string]bucks.Role{
		reader: bucks.Reader,
		writer: bucks.Writer,
	})
	require.NoError(t, err)
	assert.Len(t, all, 2)
	assert.Equal(t, bucks.Reader, all[reader])
	assert.Equal(t, bucks.Writer, all[writer])

	all, err = buck.PushPathAccessRoles(context.Background(), "file", map[string]bucks.Role{
		reader: bucks.None,
	})
	require.NoError(t, err)
	assert.Len(t, all, 1)
	_, ok := all[reader]
	assert.False(t, ok)
}

func addRandomFile(t *testing.T, buck *Bucket, pth string, size int64) string {
	bp, err := buck.Path()
	require.NoError(t, err)
	name := filepath.Join(bp, pth)
	err = os.MkdirAll(filepath.Dir(name), os.ModePerm)
	require.NoError(t, err)
	fa, err := os.Create(filepath.Join(bp, pth))
	require.NoError(t, err)
	defer fa.Close()
	_, err = io.CopyN(fa, rand.Reader, size)
	require.NoError(t, err)
	return name
}
