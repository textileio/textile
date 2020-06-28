package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/ipfs/interface-go-ipfs-core/options"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBucket(t *testing.T) {
	makeBucket(t, "testdata/a", options.BalancedLayout)
}

func TestBucket_SetCidVersion(t *testing.T) {
	buck0 := makeBucket(t, "testdata/a", options.BalancedLayout)
	buck0.SetCidVersion(0)
	err := buck0.Save(context.Background())
	require.Nil(t, err)
	lc0, _, err := buck0.Root()
	require.Nil(t, err)
	assert.True(t, lc0.Defined())
	assert.Equal(t, 0, buck0.cidver)
	assert.Equal(t, 0, int(lc0.Version()))
	buck0.Close()

	buck1 := makeBucket(t, "testdata/a", options.BalancedLayout)
	defer buck1.Close()
	buck1.SetCidVersion(1)
	err = buck1.Save(context.Background())
	require.Nil(t, err)
	lc1, _, err := buck1.Root()
	require.Nil(t, err)
	assert.True(t, lc1.Defined())
	assert.Equal(t, 1, buck1.cidver)
	assert.Equal(t, 1, int(lc1.Version()))
}

func TestBucket_Save(t *testing.T) {
	buck := makeBucket(t, "testdata/a", options.BalancedLayout)

	err := buck.Save(context.Background())
	require.Nil(t, err)
	lc, _, err := buck.Root()
	require.Nil(t, err)
	assert.True(t, lc.Defined())
	buck.Close()

	buck2 := makeBucket(t, "testdata/a", options.BalancedLayout)
	defer buck2.Close()
	n, err := buck2.Get(context.Background(), lc)
	require.Nil(t, err)
	checkLinks(t, buck2, n)

	diff, err := buck2.Diff(context.Background(), "testdata/a")
	require.Nil(t, err)
	require.Empty(t, diff)
}

func TestBucket_SaveFile(t *testing.T) {
	buck := makeBucket(t, "testdata/c", options.BalancedLayout)

	err := buck.SaveFile(context.Background(), "testdata/c/one.jpg", "one.jpg")
	require.Nil(t, err)
	lc, _, err := buck.Root()
	require.Nil(t, err)
	assert.True(t, lc.Defined())
	buck.Close()

	buck2 := makeBucket(t, "testdata/c", options.BalancedLayout)
	defer buck2.Close()
	n, err := buck2.Get(context.Background(), lc)
	require.Nil(t, err)
	checkLinks(t, buck2, n)

	diff, err := buck2.Diff(context.Background(), "testdata/c")
	require.Nil(t, err)
	require.Equal(t, 1, len(diff))
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
	buck := makeBucket(t, "testdata/c", options.BalancedLayout)
	defer buck.Close()

	c, err := buck.HashFile("testdata/c/one.jpg")
	require.Nil(t, err)
	assert.NotEmpty(t, c)
}

func TestBucket_Diff(t *testing.T) {
	buck := makeBucket(t, "testdata/a", options.BalancedLayout)
	defer buck.Close()

	err := buck.Save(context.Background())
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

func TestBucket_Get(t *testing.T) {
	buck := makeBucket(t, "testdata/a", options.BalancedLayout)
	defer buck.Close()

	err := buck.Save(context.Background())
	require.Nil(t, err)

	lc, _, err := buck.Root()
	require.Nil(t, err)
	assert.True(t, lc.Defined())
	n, err := buck.Get(context.Background(), lc)
	require.Nil(t, err)
	assert.Equal(t, lc, n.Cid())
}

func TestBucket_SetRemotePath(t *testing.T) {
	buck := makeBucket(t, "testdata/a", options.BalancedLayout)
	defer buck.Close()

	err := buck.Save(context.Background())
	require.Nil(t, err)
	lc, _, err := buck.Root()
	require.Nil(t, err)
	assert.True(t, lc.Defined())

	err = buck.SetRemotePath("", lc)
	require.Nil(t, err)
}

func TestBucket_MatchPath(t *testing.T) {
	buck := makeBucket(t, "testdata/a", options.BalancedLayout)
	defer buck.Close()

	err := buck.Save(context.Background())
	require.Nil(t, err)
	lc, _, err := buck.Root()
	require.Nil(t, err)
	assert.True(t, lc.Defined())

	rc := makeCid(t, "remote")
	err = buck.SetRemotePath("", rc)
	require.Nil(t, err)

	match, err := buck.MatchPath("", lc, rc)
	require.Nil(t, err)
	assert.True(t, match)
}

func TestBucket_RemovePath(t *testing.T) {
	buck := makeBucket(t, "testdata/a", options.BalancedLayout)
	defer buck.Close()

	k, err := getPathKey("path/to/file")
	require.Nil(t, err)
	err = buck.putPathMap(k, pathMap{
		Local:  makeCid(t, "local"),
		Remote: makeCid(t, "remote"),
	})
	require.Nil(t, err)
	err = buck.RemovePath(context.Background(), "path/to/file")
	require.Nil(t, err)

	_, err = buck.getPathMap(k)
	require.NotNil(t, err)
}

func makeBucket(t *testing.T, root string, layout options.Layout) *Bucket {
	buck, err := NewBucket(root, layout)
	require.Nil(t, err)

	t.Cleanup(func() {
		err := os.RemoveAll(filepath.Join(buck.path, filepath.Dir(repoPath)))
		require.Nil(t, err)
	})
	return buck
}

func makeCid(t *testing.T, s string) cid.Cid {
	h1, err := mh.Sum([]byte(s), mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	return cid.NewCidV1(0x55, h1)
}
