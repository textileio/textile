package local

import (
	"context"
	"io/ioutil"
	"testing"

	ipld "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBucket(t *testing.T) {
	buck := makeBucket(t)
	defer buck.Close()
}

func TestBucket_Save(t *testing.T) {
	t.Parallel()
	buck := makeBucket(t)
	defer buck.Close()

	saved, err := buck.Save(context.Background(), "testdata", options.BalancedLayout)
	require.Nil(t, err)
	assert.True(t, saved.Cid().Defined())
	t.Logf("Saved dir with cid: %s", saved.Cid())

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
	buck := makeBucket(t)
	defer buck.Close()

	saved, err := buck.Save(context.Background(), "testdata", options.BalancedLayout)
	require.Nil(t, err)
	assert.True(t, saved.Cid().Defined())

	got, err := buck.Get(context.Background(), saved.Cid())
	require.Nil(t, err)
	assert.Equal(t, got.Cid(), saved.Cid())
}

func TestBucket_Close(t *testing.T) {
	buck := makeBucket(t)
	err := buck.Close()
	require.Nil(t, err)
}

func makeBucket(t *testing.T) *bucket {
	dir, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	buck, err := NewBucket(dir, true)
	require.Nil(t, err)
	return buck
}
