package local_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alecthomas/jsonschema"
	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/db"
	bucks "github.com/textileio/textile/buckets"
	. "github.com/textileio/textile/buckets/local"
)

func TestBucket_Key(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	conf := getConf(t, buckets)
	buck, err := buckets.NewBucket(context.Background(), conf)
	require.Nil(t, err)

	assert.NotEmpty(t, buck.Key())
}

func TestBucket_Thread(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	conf := getConf(t, buckets)
	buck, err := buckets.NewBucket(context.Background(), conf)
	require.Nil(t, err)

	tid, err := buck.Thread()
	require.Nil(t, err)
	assert.Equal(t, conf.Thread, tid)
}

func TestBucket_Path(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	conf := getConf(t, buckets)
	buck, err := buckets.NewBucket(context.Background(), conf)
	require.Nil(t, err)

	bp, err := buck.Path()
	require.Nil(t, err)
	assert.Equal(t, conf.Path, bp)
}

func TestBucket_Info(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	conf := getConf(t, buckets)
	buck, err := buckets.NewBucket(context.Background(), conf, WithName("bucky"))
	require.Nil(t, err)

	info, err := buck.Info(context.Background())
	require.Nil(t, err)
	assert.NotEmpty(t, info.Key)
	assert.Equal(t, info.Name, "bucky")
	assert.NotEmpty(t, info.Path)
	assert.Equal(t, info.Thread, conf.Thread)
	assert.NotEmpty(t, info.CreatedAt)
	assert.NotEmpty(t, info.UpdatedAt)
}

func TestBucket_Roots(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.Nil(t, err)

	roots, err := buck.Roots(context.Background())
	require.Nil(t, err)
	assert.True(t, roots.Local.Defined())
	assert.True(t, roots.Remote.Defined())
}

func TestBucket_RemoteLinks(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.Nil(t, err)

	links, err := buck.RemoteLinks(context.Background())
	require.Nil(t, err)
	assert.NotEmpty(t, links.URL)
	assert.NotEmpty(t, links.IPNS)
}

func TestBucket_DBInfo(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.Nil(t, err)

	info, err := buck.DBInfo(context.Background())
	require.Nil(t, err)
	assert.True(t, info.Key.Defined())
	assert.NotEmpty(t, info.Addrs)
}

func TestBucket_PushLocal(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.Nil(t, err)

	_, err = buck.PushLocal(context.Background())
	require.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrUpToDate))

	addRandomFile(t, buck, "file", 1024)

	roots, err := buck.PushLocal(context.Background())
	require.Nil(t, err)
	assert.True(t, roots.Local.Defined())
	assert.True(t, roots.Remote.Defined())

	_, err = buck.PushLocal(context.Background())
	require.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrUpToDate))
}

func TestBucket_ListRemotePath(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.Nil(t, err)

	items, err := buck.ListRemotePath(context.Background(), "")
	require.Nil(t, err)
	assert.Len(t, items, 1)

	addRandomFile(t, buck, "dir/file", 1024)
	_, err = buck.PushLocal(context.Background())
	require.Nil(t, err)

	items, err = buck.ListRemotePath(context.Background(), "")
	require.Nil(t, err)
	assert.Len(t, items, 2)

	items, err = buck.ListRemotePath(context.Background(), "dir")
	require.Nil(t, err)
	assert.Len(t, items, 1)
}

func TestBucket_PullRemote(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.Nil(t, err)

	_, err = buck.PullRemote(context.Background())
	require.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrUpToDate))

	fpth := addRandomFile(t, buck, "dir/file", 1024)
	_, err = buck.PushLocal(context.Background())
	require.Nil(t, err)

	// Delete the file locally
	err = os.RemoveAll(fpth)
	require.Nil(t, err)

	roots, err := buck.PullRemote(context.Background())
	require.Nil(t, err)
	assert.True(t, roots.Local.Defined())
	assert.True(t, roots.Remote.Defined())

	// The local removal should be maintained
	diff, err := buck.DiffLocal()
	require.Nil(t, err)
	assert.Len(t, diff, 1)
	if len(diff) > 0 {
		assert.Equal(t, diff[0].Type, dagutils.Remove)
	}

	// Pulling hard should reset the local to the exact state of the remote
	_, err = buck.PullRemote(context.Background(), WithHard(true))
	require.Nil(t, err)

	_, err = buck.PullRemote(context.Background())
	require.NotNil(t, err)
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
	require.Nil(t, err)
	_, err = buckets.NewBucket(context.Background(), conf2)
	require.Nil(t, err)

	// Use a bucket from a nested folder to test if the relative path matching is working
	buck2, err := buckets.GetLocalBucket(context.Background(), filepath.Join(conf2.Path, "dir"))
	require.Nil(t, err)

	// Delete the file locally from the first bucket
	err = os.RemoveAll(fpth)
	require.Nil(t, err)
	_, err = buck.PushLocal(context.Background())
	require.Nil(t, err)

	// Stage a modification to the same file in the second bucket
	addRandomFile(t, buck2, "dir/file", 1024)

	// Pull the remote, the local change should be cleared
	// with only one remove event
	events := make(chan PathEvent)
	defer close(events)
	ec := &eventCollector{}
	go ec.collect(events)
	_, err = buck2.PullRemote(context.Background(), WithHard(true), WithPathEvents(events))
	require.Nil(t, err)
	ec.check(t, 0, 1)
}

func TestBucket_CatRemotePath(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.Nil(t, err)

	addRandomFile(t, buck, "file", 1024)
	_, err = buck.PushLocal(context.Background())
	require.Nil(t, err)

	var buf bytes.Buffer
	err = buck.CatRemotePath(context.Background(), "file", &buf)
	require.Nil(t, err)
	assert.Equal(t, 1024, buf.Len())
}

func TestBucket_EncryptLocalPath(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.Nil(t, err)

	fpth := addRandomFile(t, buck, "plaintext", 1024)

	var buf bytes.Buffer
	err = buck.EncryptLocalPath(fpth, "shhhhh!", &buf)
	require.Nil(t, err)
	assert.NotEmpty(t, buf)
}

func TestBucket_DecryptLocalPath(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.Nil(t, err)

	fpth := addRandomFile(t, buck, "plaintext", 1024)
	cipher := filepath.Join(filepath.Dir(fpth), "ciphertext")
	f, err := os.Create(cipher)
	require.Nil(t, err)
	err = buck.EncryptLocalPath(fpth, "shhhhh!", f)
	require.Nil(t, err)
	f.Close()

	var buf bytes.Buffer
	err = buck.DecryptLocalPath(cipher, "badpass", &buf)
	require.NotNil(t, err)

	err = buck.DecryptLocalPath(cipher, "shhhhh!", &buf)
	require.Nil(t, err)
	assert.Equal(t, 1024, buf.Len())
}

func TestBucket_DecryptRemotePath(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	buck, err := buckets.NewBucket(context.Background(), getConf(t, buckets))
	require.Nil(t, err)

	fpth := addRandomFile(t, buck, "plaintext", 1024)
	cipher := filepath.Join(filepath.Dir(fpth), "ciphertext")
	f, err := os.Create(cipher)
	require.Nil(t, err)
	defer f.Close()
	err = buck.EncryptLocalPath(fpth, "shhhhh!", f)
	require.Nil(t, err)
	err = os.RemoveAll(fpth)
	_, err = buck.PushLocal(context.Background())
	require.Nil(t, err)

	var buf bytes.Buffer
	err = buck.DecryptRemotePath(context.Background(), "ciphertext", "badpass", &buf)
	require.NotNil(t, err)

	err = buck.DecryptRemotePath(context.Background(), "ciphertext", "shhhhh!", &buf)
	require.Nil(t, err)

	assert.Equal(t, 1024, buf.Len())
}

func TestBucket_AddRemoteCid(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	conf := getConf(t, buckets)
	buck, err := buckets.NewBucket(context.Background(), conf)
	require.Nil(t, err)

	pth := createIpfsFolder(t)
	err = buck.AddRemoteCid(context.Background(), pth.Cid(), conf.Path)
	require.Nil(t, err)

	bp, err := buck.Path()
	require.Nil(t, err)
	_, err = os.Stat(filepath.Join(bp, "file1.txt"))
	require.Nil(t, err)
	_, err = os.Stat(filepath.Join(bp, "folder1", "file2.txt"))
	require.Nil(t, err)
}

func TestBucket_DiffLocal(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	conf := getConf(t, buckets)
	buck, err := buckets.NewBucket(context.Background(), conf)
	require.Nil(t, err)

	addRandomFile(t, buck, "file1", 256)
	fpth := addRandomFile(t, buck, "folder/file2", 256)
	diff, err := buck.DiffLocal()
	require.Nil(t, err)
	assert.Len(t, diff, 2)
	assert.Equal(t, diff[0].Type, dagutils.Add)

	// Test diff from a nested folder
	buck2, err := buckets.GetLocalBucket(context.Background(), filepath.Join(conf.Path, "folder"))
	require.Nil(t, err)
	diff, err = buck2.DiffLocal()
	require.Nil(t, err)
	assert.Len(t, diff, 2)
	assert.Equal(t, diff[0].Type, dagutils.Add)

	_, err = buck.PushLocal(context.Background())
	require.Nil(t, err)

	diff, err = buck.DiffLocal()
	require.Nil(t, err)
	assert.Len(t, diff, 0)

	addRandomFile(t, buck, "file1", 256)
	diff, err = buck.DiffLocal()
	require.Nil(t, err)
	assert.Len(t, diff, 1)
	assert.Equal(t, diff[0].Type, dagutils.Mod)

	_, err = buck.PushLocal(context.Background())
	require.Nil(t, err)

	diff, err = buck.DiffLocal()
	require.Nil(t, err)
	assert.Len(t, diff, 0)

	err = os.RemoveAll(fpth)
	require.Nil(t, err)
	diff, err = buck.DiffLocal()
	require.Nil(t, err)
	assert.Len(t, diff, 1)
	assert.Equal(t, diff[0].Type, dagutils.Remove)

	_, err = buck.PushLocal(context.Background())
	require.Nil(t, err)

	diff, err = buck.DiffLocal()
	require.Nil(t, err)
	assert.Len(t, diff, 0)
}

func TestBucket_Watch(t *testing.T) {
	t.Parallel()
	buckets1 := setup(t)
	conf := getConf(t, buckets1)
	buck1, err := buckets1.NewBucket(context.Background(), conf)
	require.Nil(t, err)

	events := make(chan PathEvent)
	defer close(events)
	ec := &eventCollector{}
	go ec.collect(events)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		err = buck1.Watch(ctx, WithWatchEvents(events))
		require.Nil(t, err)
	}()

	// Add a file
	addRandomFile(t, buck1, "file1", 512)

	// Wait a sec while the watcher kicks off a watch cycle
	time.Sleep(time.Second)
	// Watch should have handled the diff
	_, err = buck1.PushLocal(context.Background())
	require.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrUpToDate))

	// Create the same bucket from a different daemon
	buckets2 := setup(t)
	tid, err := buck1.Thread()
	require.Nil(t, err)
	dbinfo, err := buckets1.Clients().Threads.GetDBInfo(context.Background(), tid)
	require.Nil(t, err)
	colinfo, err := buckets1.Clients().Threads.GetCollectionInfo(context.Background(), tid, bucks.CollectionName)
	require.Nil(t, err)

	schema := &jsonschema.Schema{}
	err = json.Unmarshal(colinfo.Schema, schema)
	require.Nil(t, err)

	err = buckets2.Clients().Threads.NewDBFromAddr(
		context.Background(),
		dbinfo.Addrs[0],
		dbinfo.Key,
		db.WithNewManagedName(dbinfo.Name),
		db.WithNewManagedCollections(db.CollectionConfig{
			Name:    bucks.CollectionName,
			Schema:  schema,
			Indexes: colinfo.Indexes,
		}),
		db.WithNewManagedBackfillBlock(true))
	require.Nil(t, err)

	buck2, err := buckets2.NewBucket(context.Background(), Config{
		Path:   newDir(t),
		Key:    buck1.Key(),
		Thread: tid,
	})
	require.Nil(t, err)

	// Add another file to the second bucket
	addRandomFile(t, buck2, "file2", 512)
	_, err = buck2.PushLocal(context.Background())
	require.Nil(t, err)

	// Wait a sec while the remote event is handled
	time.Sleep(time.Second)
	// Watch should have handled the remote diff
	_, err = buck1.PullRemote(context.Background())
	require.NotNil(t, err)
	assert.True(t, errors.Is(err, ErrUpToDate))

	bp, err := buck1.Path()
	require.Nil(t, err)
	_, err = os.Stat(filepath.Join(bp, "file2"))
	require.Nil(t, err)

	ec.check(t, 2, 0)
}

func TestBucket_Destroy(t *testing.T) {
	t.Parallel()
	buckets := setup(t)
	conf := getConf(t, buckets)
	buck, err := buckets.NewBucket(context.Background(), conf)
	require.Nil(t, err)

	err = buck.Destroy(context.Background())
	require.Nil(t, err)

	// Ensure the local bucket was removed
	_, err = buckets.GetLocalBucket(context.Background(), conf.Path)
	require.NotNil(t, err)

	// Ensure the remote bucket was removed
	list, err := buckets.RemoteBuckets(context.Background())
	require.Nil(t, err)
	assert.Len(t, list, 0)
}

func addRandomFile(t *testing.T, buck *Bucket, pth string, size int64) string {
	bp, err := buck.Path()
	require.Nil(t, err)
	name := filepath.Join(bp, pth)
	err = os.MkdirAll(filepath.Dir(name), os.ModePerm)
	require.Nil(t, err)
	fa, err := os.Create(filepath.Join(bp, pth))
	require.Nil(t, err)
	defer fa.Close()
	_, err = io.CopyN(fa, rand.Reader, size)
	require.Nil(t, err)
	return name
}
