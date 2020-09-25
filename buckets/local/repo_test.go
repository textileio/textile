package local_test

import (
	"context"
	"crypto/rand"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/ipfs/go-cid"
	ipld "github.com/ipfs/go-ipld-format"
	du "github.com/ipfs/go-merkledag/dagutils"
	"github.com/ipfs/interface-go-ipfs-core/options"
	mh "github.com/multiformats/go-multihash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	. "github.com/textileio/textile/v2/buckets/local"
)

func TestNewRepo(t *testing.T) {
	makeRepo(t, "testdata/a", options.BalancedLayout)
}

func TestRepo_SetCidVersion(t *testing.T) {
	repo0 := makeRepo(t, "testdata/a", options.BalancedLayout)
	repo0.SetCidVersion(0)
	err := repo0.Save(context.Background())
	require.NoError(t, err)
	lc0, _, err := repo0.Root()
	require.NoError(t, err)
	assert.True(t, lc0.Defined())
	assert.Equal(t, 0, repo0.CidVersion())
	assert.Equal(t, 0, int(lc0.Version()))
	repo0.Close()

	repo1 := makeRepo(t, "testdata/a", options.BalancedLayout)
	defer repo1.Close()
	repo1.SetCidVersion(1)
	err = repo1.Save(context.Background())
	require.NoError(t, err)
	lc1, _, err := repo1.Root()
	require.NoError(t, err)
	assert.True(t, lc1.Defined())
	assert.Equal(t, 1, repo1.CidVersion())
	assert.Equal(t, 1, int(lc1.Version()))
}

func TestRepo_Save(t *testing.T) {
	repo := makeRepo(t, "testdata/a", options.BalancedLayout)

	err := repo.Save(context.Background())
	require.NoError(t, err)
	lc, _, err := repo.Root()
	require.NoError(t, err)
	assert.True(t, lc.Defined())
	repo.Close()

	repo2 := makeRepo(t, "testdata/a", options.BalancedLayout)
	defer repo2.Close()
	n, err := repo2.GetNode(context.Background(), lc)
	require.NoError(t, err)
	checkLinks(t, repo2, n)

	diff, err := repo2.Diff(context.Background(), "testdata/a")
	require.NoError(t, err)
	require.Empty(t, diff)
}

func TestRepo_SaveFile(t *testing.T) {
	repo := makeRepo(t, "testdata/c", options.BalancedLayout)

	err := repo.SaveFile(context.Background(), "testdata/c/one.jpg", "one.jpg")
	require.NoError(t, err)
	lc, _, err := repo.Root()
	require.NoError(t, err)
	assert.True(t, lc.Defined())
	repo.Close()

	repo2 := makeRepo(t, "testdata/c", options.BalancedLayout)
	defer repo2.Close()
	n, err := repo2.GetNode(context.Background(), lc)
	require.NoError(t, err)
	checkLinks(t, repo2, n)

	diff, err := repo2.Diff(context.Background(), "testdata/c")
	require.NoError(t, err)
	require.Equal(t, 1, len(diff))
}

func checkLinks(t *testing.T, repo *Repo, n ipld.Node) {
	for _, l := range n.Links() {
		ln, err := repo.GetNode(context.Background(), l.Cid)
		if l.Name == "" { // Data node should not have been saved
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			checkLinks(t, repo, ln)
		}
	}
}

func TestRepo_HashFile(t *testing.T) {
	repo := makeRepo(t, "testdata/c", options.BalancedLayout)
	defer repo.Close()

	c, err := repo.HashFile("testdata/c/one.jpg")
	require.NoError(t, err)
	assert.NotEmpty(t, c)
}

func TestRepo_Diff(t *testing.T) {
	t.Run("raw files", func(t *testing.T) {
		repo := makeRepo(t, "testdata/a", options.BalancedLayout)
		defer repo.Close()

		err := repo.Save(context.Background())
		require.NoError(t, err)

		diffa, err := repo.Diff(context.Background(), "testdata/a")
		require.NoError(t, err)
		assert.Empty(t, diffa)

		diffb, err := repo.Diff(context.Background(), "testdata/b")
		require.NoError(t, err)
		assert.NotEmpty(t, diffb)
		assert.Equal(t, 6, len(diffb))

		changes := []*du.Change{
			{Path: "foo.txt", Type: du.Mod},
			{Path: "one/two/boo.txt", Type: du.Remove},
			{Path: "one/buz.txt", Type: du.Remove},
			{Path: "one/muz.txt", Type: du.Add},
			{Path: "one/three", Type: du.Add},
			{Path: "bar.txt", Type: du.Remove},
		}
		for i, c := range diffb {
			assert.Equal(t, changes[i].Path, c.Path)
			assert.Equal(t, changes[i].Type, c.Type)
		}
	})

	t.Run("proto files", func(t *testing.T) {
		var dirs []string
		t.Cleanup(func() {
			for _, d := range dirs {
				_ = os.RemoveAll(d)
			}
		})

		dira, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		dirs = append(dirs, dira)

		repo := makeRepo(t, dira, options.BalancedLayout)
		defer repo.Close()

		// Add a file large enough to have multiple raw node chunks
		fa, err := os.Create(filepath.Join(dira, "file"))
		require.NoError(t, err)
		defer fa.Close()
		_, err = io.CopyN(fa, rand.Reader, 1000*1024)
		require.NoError(t, err)

		err = repo.Save(context.Background())
		require.NoError(t, err)

		diffa, err := repo.Diff(context.Background(), dira)
		require.NoError(t, err)
		assert.Empty(t, diffa)

		// Create another dir with a file of the same name.
		// We should see the file as modified in the diff.
		dirb, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		dirs = append(dirs, dirb)
		fb, err := os.Create(filepath.Join(dirb, "file"))
		require.NoError(t, err)
		defer fb.Close()
		_, err = io.CopyN(fb, rand.Reader, 1000*1024)
		require.NoError(t, err)

		diffb, err := repo.Diff(context.Background(), dirb)
		require.NoError(t, err)
		assert.NotEmpty(t, diffb)
		assert.Equal(t, 1, len(diffb))
		changes := []*du.Change{
			{Path: filepath.Base(fa.Name()), Type: du.Mod},
		}
		for i, c := range diffb {
			assert.Equal(t, changes[i].Path, c.Path)
			assert.Equal(t, changes[i].Type, c.Type)
		}

		// Create another dir with a nested dir of the same name as the original file.
		// We should see the file as modified in the diff.
		dirc, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		dirs = append(dirs, dirc)
		err = os.MkdirAll(filepath.Join(dirc, "file"), os.ModePerm)
		require.NoError(t, err)
		fc, err := os.Create(filepath.Join(dirc, "file", "file"))
		require.NoError(t, err)
		defer fc.Close()
		_, err = io.CopyN(fb, rand.Reader, 1000*1024)
		require.NoError(t, err)

		diffc, err := repo.Diff(context.Background(), dirc)
		require.NoError(t, err)
		assert.NotEmpty(t, diffc)
		assert.Equal(t, 1, len(diffc))
		for i, c := range diffc {
			assert.Equal(t, changes[i].Path, c.Path)
			assert.Equal(t, changes[i].Type, c.Type)
		}
	})
}

func TestRepo_Get(t *testing.T) {
	repo := makeRepo(t, "testdata/a", options.BalancedLayout)
	defer repo.Close()

	err := repo.Save(context.Background())
	require.NoError(t, err)

	lc, _, err := repo.Root()
	require.NoError(t, err)
	assert.True(t, lc.Defined())
	n, err := repo.GetNode(context.Background(), lc)
	require.NoError(t, err)
	assert.Equal(t, lc, n.Cid())
}

func TestRepo_SetRemotePath(t *testing.T) {
	repo := makeRepo(t, "testdata/a", options.BalancedLayout)
	defer repo.Close()

	err := repo.Save(context.Background())
	require.NoError(t, err)
	lc, _, err := repo.Root()
	require.NoError(t, err)
	assert.True(t, lc.Defined())

	err = repo.SetRemotePath("", lc)
	require.NoError(t, err)
}

func TestRepo_MatchPath(t *testing.T) {
	repo := makeRepo(t, "testdata/a", options.BalancedLayout)
	defer repo.Close()

	err := repo.Save(context.Background())
	require.NoError(t, err)
	lc, _, err := repo.Root()
	require.NoError(t, err)
	assert.True(t, lc.Defined())

	rc := makeCid(t, "remote")
	err = repo.SetRemotePath("", rc)
	require.NoError(t, err)

	match, err := repo.MatchPath("", lc, rc)
	require.NoError(t, err)
	assert.True(t, match)
}

func TestRepo_RemovePath(t *testing.T) {
	repo := makeRepo(t, "testdata/a", options.BalancedLayout)
	defer repo.Close()

	err := repo.SetRemotePath("path/to/file", makeCid(t, "remote"))
	require.NoError(t, err)
	err = repo.RemovePath(context.Background(), "path/to/file")
	require.NoError(t, err)

	_, _, err = repo.GetPathMap("path/to/file")
	require.Error(t, err)
}

func makeRepo(t *testing.T, root string, layout options.Layout) *Repo {
	repo, err := NewRepo(root, ".textile/repo", layout)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := os.RemoveAll(repo.Path())
		require.NoError(t, err)
	})
	return repo
}

func makeCid(t *testing.T, s string) cid.Cid {
	h1, err := mh.Sum([]byte(s), mh.SHA2_256, -1)
	if err != nil {
		t.Fatal(err)
	}
	return cid.NewCidV1(0x55, h1)
}
