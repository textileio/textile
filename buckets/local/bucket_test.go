package local

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	"github.com/ipfs/go-merkledag/dagutils"

	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBucket(t *testing.T) {
	buck := makeBucket(t, options.BalancedLayout)
	defer buck.Close()
}

func TestBucket_Save(t *testing.T) {
	t.Parallel()
	buck := makeBucket(t, options.BalancedLayout)
	defer buck.Close()

	saved, err := buck.Save(context.Background(), "testdata/a")
	require.Nil(t, err)
	assert.True(t, saved.Cid().Defined())
	checkLinks(t, buck, saved)
}

func checkLinks(t *testing.T, buck *bucket, n ipld.Node) {
	for _, l := range n.Links() {
		ln, err := buck.Get(context.Background(), l.Cid)
		if l.Name == "" { // Data node should not have been saved
			require.NotNil(t, err)
		} else {
			require.Nil(t, err)
			checkLinks(t, buck, ln)
		}
	}
}

func TestBucket_Get(t *testing.T) {
	t.Parallel()
	buck := makeBucket(t, options.BalancedLayout)
	defer buck.Close()

	saved, err := buck.Save(context.Background(), "testdata/a")
	require.Nil(t, err)
	got, err := buck.Get(context.Background(), saved.Cid())
	require.Nil(t, err)
	assert.Equal(t, got.Cid(), saved.Cid())
}

func TestBucket_Diff(t *testing.T) {
	t.Parallel()
	buck := makeBucket(t, options.BalancedLayout)
	defer buck.Close()

	a, err := buck.Save(context.Background(), "testdata/a")
	require.Nil(t, err)

	diffa, err := buck.Diff(context.Background(), a.Cid(), "testdata/a")
	require.Nil(t, err)
	assert.Empty(t, diffa)

	diffb, err := buck.Diff(context.Background(), a.Cid(), "testdata/b")
	require.Nil(t, err)
	assert.NotEmpty(t, diffb)
	assert.Equal(t, 6, len(diffb))

	changes := []*dagutils.Change{
		{Path: "foo.txt", Type: dagutils.Mod},
		{Path: "one/two/boo.txt", Type: dagutils.Remove},
		{Path: "one/buz.txt", Type: dagutils.Remove},
		{Path: "one/muz.txt", Type: dagutils.Add},
		{Path: "one/three", Type: dagutils.Add},
		{Path: "bar.txt", Type: dagutils.Remove},
	}
	for i, c := range diffb {
		assert.Equal(t, changes[i].Path, c.Path)
		assert.Equal(t, changes[i].Type, c.Type)
	}
}

func TestBucket_Write(t *testing.T) {
	t.Parallel()
	buck := makeBucket(t, options.BalancedLayout)
	defer buck.Close()

	saved, err := buck.Save(context.Background(), "testdata/a")
	require.Nil(t, err)

	buf := new(bytes.Buffer)
	err = buck.Write(context.Background(), saved.Cid(), buf)
	require.Nil(t, err)
}

func TestBucket_Load(t *testing.T) {
	t.Parallel()
	buck := makeBucket(t, options.BalancedLayout)
	defer buck.Close()

	saved, err := buck.Save(context.Background(), "testdata/c")
	require.Nil(t, err)

	buf := new(bytes.Buffer)
	err = buck.Write(context.Background(), saved.Cid(), buf)
	require.Nil(t, err)

	loaded, err := buck.Load(buf)
	require.Nil(t, err)
	require.Equal(t, saved.Cid(), loaded)
}

func TestBucket_Close(t *testing.T) {
	buck := makeBucket(t, options.BalancedLayout)
	err := buck.Close()
	require.Nil(t, err)
}

func makeBucket(t *testing.T, layout options.Layout) *bucket {
	dir, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	buck, err := NewBucket(dir, layout, true)
	require.Nil(t, err)
	return buck
}
