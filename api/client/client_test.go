package client

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/textile/core"
)

var (
	addrApi = parseAddr("/ip4/127.0.0.1/tcp/3006")
)

func TestLogin(t *testing.T) {
	shutdown, err := makeTextile()
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	client, err := NewClient(addrApi)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test login", func(t *testing.T) {
		id, err := client.Login(context.Background(), "jon@doe.com")
		if err != nil {
			t.Fatalf("failed to login: %v", err)
		}
		if id == "" {
			t.Fatal("got empty id from login")
		}
	})
}

func TestClose(t *testing.T) {
	shutdown, err := makeTextile()
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	client, err := NewClient(addrApi)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test close", func(t *testing.T) {
		if err := client.Close(); err != nil {
			t.Fatalf("failed to close client: %v", err)
		}
	})
}

func makeTextile() (func(), error) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, err
	}

	textile, err := core.NewTextile(core.Config{
		RepoPath:             dir,
		AddrApi:              addrApi,
		AddrThreadsHost:      parseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrThreadsHostProxy: parseAddr("/ip4/0.0.0.0/tcp/0"),
		// @todo: Currently, this can't be port zero because the client would not
		// know the randomly chosen listen port.
		AddrThreadsApi:      parseAddr("/ip4/127.0.0.1/tcp/6006"),
		AddrThreadsApiProxy: parseAddr("/ip4/127.0.0.1/tcp/0"),
		AddrIpfsApi:         parseAddr("/ip4/127.0.0.1/tcp/5001"),
		Debug:               true,
	})
	if err != nil {
		return nil, err
	}
	textile.Bootstrap()

	return func() {
		textile.Close()
		_ = os.RemoveAll(dir)
	}, nil
}

func parseAddr(str string) ma.Multiaddr {
	addr, err := ma.NewMultiaddr(str)
	if err != nil {
		panic(err)
	}
	return addr
}

func checkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
