package client

import (
	"io/ioutil"
	"os"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/textile/core"
)

var (
	shutdown func()
	client   *Client

	addrApi = parseAddr("/ip4/127.0.0.1/tcp/3006")
)

func TestMain(m *testing.M) {
	var err error
	shutdown = makeTextile()
	client, err = NewClient(addrApi)
	if err != nil {
		panic(err)
	}
	exitVal := m.Run()
	shutdown()
	os.Exit(exitVal)
}

func TestSignUp(t *testing.T) {
	id, err := client.SignUp()
	if err != nil {
		t.Fatalf("failed to sign up: %v", err)
	}
	if id == "" {
		t.Fatal("got empty id from sign up")
	}
}

func TestClose(t *testing.T) {
	err := client.Close()
	if err != nil {
		t.Fatalf("failed to close client: %v", err)
	}
}

func makeTextile() func() {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		panic(err)
	}

	textile, err := core.NewTextile(core.Config{
		RepoPath:             dir,
		AddrApi:              addrApi,
		AddrThreadsHost:      parseAddr("/ip4/0.0.0.0/tcp/4006"),
		AddrThreadsHostProxy: parseAddr("/ip4/0.0.0.0/tcp/5006"),
		AddrThreadsApi:       parseAddr("/ip4/127.0.0.1/tcp/6006"),
		AddrThreadsApiProxy:  parseAddr("/ip4/127.0.0.1/tcp/7006"),
		AddrIpfsApi:          parseAddr("/ip4/127.0.0.1/tcp/5001"),
		Debug:                true,
	})
	if err != nil {
		panic(err)
	}
	textile.Bootstrap()

	return func() {
		textile.Close()
		_ = os.RemoveAll(dir)
	}
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
