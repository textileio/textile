package client_test

/*
import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	tutil "github.com/textileio/go-threads/util"
	c "github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/core"
)

func TestClient_ListBucketPath(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := Login(t, client, conf, "jon@doe.com")
	file, err := os.Open("testdata/file1.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	_, file1Root, err := client.PushBucketPath(
		context.Background(), project.Name, "mybuck1/file1.jpg", file, c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = client.PushBucketPath(
		context.Background(), project.Name, "mybuck2/file1.jpg", file,
		c.Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}

	t.Run("test list buckets", func(t *testing.T) {
		rep, err := client.ListBucketPath(context.Background(), project.Name, "", c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("list buckets should succeed: %v", err)
		}
		if len(rep.Item.Items) != 2 {
			t.Fatalf("got wrong bucket count from list buckets, expected %d, got %d", 2,
				len(rep.Item.Items))
		}
	})

	t.Run("test list bucket path", func(t *testing.T) {
		rep, err := client.ListBucketPath(context.Background(), project.Name, "mybuck1/file1.jpg",
			c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(rep.Item.Path, "file1.jpg") {
			t.Fatal("got bad name from get bucket path")
		}
		if rep.Item.IsDir {
			t.Fatal("path is not a dir")
		}
		if rep.Root.Path != file1Root.String() {
			t.Fatal("path root should match bucket root")
		}
	})
}

func TestClient_PushBucketPath(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test push bucket path", func(t *testing.T) {
		file, err := os.Open("testdata/file1.jpg")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		progress := make(chan int64)
		go func() {
			for p := range progress {
				t.Logf("progress: %d", p)
			}
		}()
		pth, root, err := client.PushBucketPath(
			context.Background(), project.Name, "mybuck/file1.jpg", file, c.Auth{Token: user.SessionID},
			c.WithPushProgress(progress))
		if err != nil {
			t.Fatalf("push bucket path should succeed: %v", err)
		}
		if pth == nil {
			t.Fatal("got bad path from push bucket path")
		}
		if root == nil {
			t.Fatal("got bad root from push bucket path")
		}
	})

	t.Run("test push nested bucket path", func(t *testing.T) {
		file, err := os.Open("testdata/file2.jpg")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		progress := make(chan int64)
		go func() {
			for p := range progress {
				t.Logf("progress: %d", p)
			}
		}()
		pth, root, err := client.PushBucketPath(
			context.Background(), project.Name, "mybuck/path/to/file2.jpg", file, c.Auth{Token: user.SessionID},
			c.WithPushProgress(progress))
		if err != nil {
			t.Fatalf("push nested bucket path should succeed: %v", err)
		}
		if pth == nil {
			t.Fatal("got bad path from push nested bucket path")
		}
		if root == nil {
			t.Fatal("got bad root from push nested bucket path")
		}

		rep, err := client.ListBucketPath(context.Background(), project.Name, "mybuck",
			c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatal(err)
		}
		if len(rep.Item.Items) != 2 {
			t.Fatalf("got wrong bucket entry count from push nested bucket path, expected %d, got %d",
				2, len(rep.Item.Items))
		}
	})
}

func TestClient_PullBucketPath(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open("testdata/file1.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, _, err := client.PushBucketPath(context.Background(), project.Name, "mybuck/file1.jpg", file,
		c.Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}

	t.Run("test pull bucket path", func(t *testing.T) {
		file, err := ioutil.TempFile("", "")
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()

		progress := make(chan int64)
		go func() {
			for p := range progress {
				t.Logf("progress: %d", p)
			}
		}()
		if err := client.PullBucketPath(
			context.Background(), "mybuck/file1.jpg", file, c.Auth{Token: user.SessionID},
			c.WithPullProgress(progress)); err != nil {
			t.Fatalf("pull bucket path should succeed: %v", err)
		}
		info, err := file.Stat()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote file with size %d", info.Size())
	})
}

func TestClient_RemoveBucketPath(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	file1, err := os.Open("testdata/file1.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer file1.Close()
	file2, err := os.Open("testdata/file2.jpg")
	if err != nil {
		t.Fatal(err)
	}
	defer file2.Close()
	if _, _, err = client.PushBucketPath(
		context.Background(), project.Name, "mybuck1/file1.jpg", file1,
		c.Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}
	if _, _, err = client.PushBucketPath(
		context.Background(), project.Name, "mybuck1/again/file2.jpg", file1,
		c.Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}

	t.Run("test remove bucket path", func(t *testing.T) {
		if err := client.RemoveBucketPath(context.Background(), "mybuck1/again/file2.jpg",
			c.Auth{Token: user.SessionID}); err != nil {
			t.Fatalf("remove bucket path should succeed: %v", err)
		}
		if _, err := client.ListBucketPath(context.Background(), project.Name, "mybuck1/again/file2.jpg",
			c.Auth{Token: user.SessionID}); err == nil {
			t.Fatal("got bucket path that should have been removed")
		}
		if _, err := client.ListBucketPath(context.Background(), project.Name, "mybuck1",
			c.Auth{Token: user.SessionID}); err != nil {
			t.Fatalf("bucket should still exist, but get failed: %v", err)
		}
	})

	if _, _, err = client.PushBucketPath(
		context.Background(), project.Name, "mybuck2/file1.jpg", file1,
		c.Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}

	t.Run("test remove entire bucket by path", func(t *testing.T) {
		if err := client.RemoveBucketPath(context.Background(), "mybuck2/file1.jpg",
			c.Auth{Token: user.SessionID}); err != nil {
			t.Fatalf("remove bucket path should succeed: %v", err)
		}
		if _, err := client.ListBucketPath(context.Background(), project.Name, "mybuck2",
			c.Auth{Token: user.SessionID}); err == nil {
			t.Fatal("got bucket that should have been removed")
		}
	})
}

func TestClose(t *testing.T) {
	t.Parallel()
	conf, shutdown := makeTextile(t)
	defer shutdown()
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrApi)
	if err != nil {
		t.Fatal(err)
	}
	client, err := c.NewClient(target, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test close", func(t *testing.T) {
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func setup(t *testing.T) (core.Config, *c.Client, func()) {
	conf, shutdown := MakeTextile(t)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrApi)
	require.Nil(t, err)
	client, err := c.NewClient(target, nil)
	require.Nil(t, err)

	return conf, client, func() {
		shutdown()
		err := client.Close()
		require.Nil(t, err)
	}
}
*/
