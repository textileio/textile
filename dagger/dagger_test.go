package dagger

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	ipld "github.com/ipfs/go-ipld-format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDagger(t *testing.T) {
	dag := makeDagger(t)
	defer dag.Close()
}

func TestDagger_HashFile(t *testing.T) {
	t.Parallel()
	dag := makeDagger(t)
	defer dag.Close()

	file, err := os.Open("testdata/file1.jpg")
	require.Nil(t, err)
	defer file.Close()
	n, err := dag.HashFile(file, Balanced)
	require.Nil(t, err)
	assert.True(t, n.Cid().Defined())
}

func TestDagger_SaveDir(t *testing.T) {
	t.Parallel()
	dag := makeDagger(t)
	defer dag.Close()

	saved, err := dag.SaveDir(context.Background(), "testdata/", Balanced)
	require.Nil(t, err)
	assert.True(t, saved.Cid().Defined())
	t.Logf("Saved dir with cid: %s", saved.Cid())

	got, err := dag.GetDir(context.Background(), saved.Cid())
	require.Nil(t, err)
	assert.Equal(t, got.Cid(), saved.Cid())

	checkLinks(t, dag, got)
}

func checkLinks(t *testing.T, dag *dagger, n ipld.Node) {
	for _, l := range n.Links() {
		ln, err := dag.GetDir(context.Background(), l.Cid)
		if l.Name == "" { // Data node should not have been saved
			require.NotNil(t, err)
		} else {
			require.Nil(t, err)
			checkLinks(t, dag, ln)
		}
	}
}

func TestDagger_Close(t *testing.T) {
	dag := makeDagger(t)
	err := dag.Close()
	require.Nil(t, err)
}

func makeDagger(t *testing.T) *dagger {
	dir, err := ioutil.TempDir("", "")
	require.Nil(t, err)
	dag, err := NewDagger(dir, true)
	require.Nil(t, err)
	return dag
}
