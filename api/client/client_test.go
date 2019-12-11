package client

import (
	"os"
	"testing"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/textile/api"
)

var (
	shutdown   func()
	client     *Client
	serverAddr = ma.Cast([]byte("/ip4/127.0.0.1/tcp/9090"))
)

func TestMain(m *testing.M) {
	var err error
	_, shutdown = makeServer()
	client, err = NewClient(serverAddr)
	if err != nil {
		panic(err)
	}
	exitVal := m.Run()
	shutdown()
	os.Exit(exitVal)
}

func TestNewUser(t *testing.T) {
	_, err := client.NewUser()
	if err != nil {
		t.Fatalf("failed to create new user: %v", err)
	}
}

func TestClose(t *testing.T) {
	err := client.Close()
	if err != nil {
		t.Fatalf("failed to close client: %v", err)
	}
}

func makeServer() (*api.Server, func()) {
	return nil, func() {}
	//dir, err := ioutil.TempDir("", "")
	//if err != nil {
	//	panic(err)
	//}
	//ts, err := es.DefaultThreadservice(
	//	dir,
	//	es.Debug(true))
	//if err != nil {
	//	panic(err)
	//}
	//ts.Bootstrap(util.DefaultBoostrapPeers())
	//apiAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/%d", clientHost, clientPort))
	//if err != nil {
	//	panic(err)
	//}
	//apiProxyAddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/tcp/0", clientHost))
	//if err != nil {
	//	panic(err)
	//}
	//server, err := api.NewServer(context.Background(), ts, api.Config{
	//	RepoPath:  dir,
	//	Addr:      apiAddr,
	//	ProxyAddr: apiProxyAddr,
	//	Debug:     true,
	//})
	//if err != nil {
	//	panic(err)
	//}
	//return server, func() {
	//	server.Close()
	//	if err := ts.Close(); err != nil {
	//		panic(err)
	//	}
	//	_ = os.RemoveAll(dir)
	//}
}

func checkErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
