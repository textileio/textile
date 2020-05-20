package local

import (
	"context"
	"testing"

	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBucket(t *testing.T) {
	makeBucket(t, "testdata/a", options.BalancedLayout)
}

func TestBucket_Root(t *testing.T) {
	t.Parallel()
	buck := makeBucket(t, "testdata/a", options.BalancedLayout)

	err := buck.Archive(context.Background())
	require.Nil(t, err)
	assert.NotEmpty(t, buck.Path())
}

func TestBucket_Get(t *testing.T) {
	t.Parallel()
	buck := makeBucket(t, "testdata/a", options.BalancedLayout)

	err := buck.Archive(context.Background())
	require.Nil(t, err)

	n, err := buck.Get(context.Background(), buck.Path().Cid())
	require.Nil(t, err)
	assert.Equal(t, buck.Path().Cid(), n.Cid())
}

func TestBucket_Archive(t *testing.T) {
	t.Parallel()
	buck := makeBucket(t, "testdata/a", options.BalancedLayout)

	err := buck.Archive(context.Background())
	require.Nil(t, err)
	assert.NotEmpty(t, buck.Path())

	buck2 := makeBucket(t, "testdata/a", options.BalancedLayout)
	n, err := buck2.Get(context.Background(), buck.Path().Cid())
	require.Nil(t, err)
	checkLinks(t, buck2, n)
}

func TestBucket_ArchiveFile(t *testing.T) {
	t.Parallel()
	buck := makeBucket(t, "testdata/c", options.BalancedLayout)

	err := buck.ArchiveFile(context.Background(), "testdata/c/one.jpg", "one.jpg")
	require.Nil(t, err)
	assert.NotEmpty(t, buck.Path())

	buck2 := makeBucket(t, "testdata/c", options.BalancedLayout)
	n, err := buck2.Get(context.Background(), buck.Path().Cid())
	require.Nil(t, err)
	checkLinks(t, buck2, n)
}

func checkLinks(t *testing.T, buck *Bucket, n ipld.Node) {
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

func TestBucket_HashFile(t *testing.T) {
	t.Parallel()
	buck := makeBucket(t, "testdata/c", options.BalancedLayout)

	c, err := buck.HashFile("testdata/c/one.jpg")
	require.Nil(t, err)
	assert.NotEmpty(t, c)
}

func TestBucket_Diff(t *testing.T) {
	t.Parallel()
	buck := makeBucket(t, "testdata/a", options.BalancedLayout)

	err := buck.Archive(context.Background())
	require.Nil(t, err)

	diffa, err := buck.Diff(context.Background(), "testdata/a")
	require.Nil(t, err)
	assert.Empty(t, diffa)

	diffb, err := buck.Diff(context.Background(), "testdata/b")
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

func makeBucket(t *testing.T, root string, layout options.Layout) *Bucket {
	buck, err := NewBucket(root, layout)
	require.Nil(t, err)
	return buck
}
