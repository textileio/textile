package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/textileio/textile/api/pb"
	"github.com/textileio/textile/core"
	"github.com/textileio/textile/util"
)

var (
	sessionSecret = "test_runner"
)

func TestLogin(t *testing.T) {
	conf, shutdown, err := makeTextile()
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	client, err := NewClient(conf.AddrApi)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test login", func(t *testing.T) {
		var res *pb.LoginReply
		go func() {
			res, err = client.Login(context.Background(), "jon@doe.com")
			if err != nil {
				t.Fatal(err)
			}
		}()

		// Ensure login request has processed
		time.Sleep(time.Second)
		verificationURL := fmt.Sprintf("%s/verify/%s", conf.AddrGatewayUrl, sessionSecret)
		if _, err := http.Get(verificationURL); err != nil {
			t.Fatalf("failed to reach gateway: %v", err)
		}

		// Ensure login response has been received
		time.Sleep(time.Second)
		if res == nil || res.Token == "" {
			t.Fatal("got empty token from login")
		}
	})
}

func TestAddTeam(t *testing.T) {
	conf, shutdown, err := makeTextile()
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	client, err := NewClient(conf.AddrApi)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test add team", func(t *testing.T) {
		if _, err := client.AddTeam(context.Background(), "foo"); err != nil {
			t.Fatalf("add team should succeed: %v", err)
		}
	})
}

func TestAddProject(t *testing.T) {
	conf, shutdown, err := makeTextile()
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	client, err := NewClient(conf.AddrApi)
	if err != nil {
		t.Fatal(err)
	}

	//user, err := client.Login(context.Background(), "jon@doe.com")
	//if err != nil {
	//	t.Fatalf("failed to login: %v", err)
	//}

	t.Run("test add project without scope", func(t *testing.T) {
		if _, err := client.AddProject(context.Background(), "foo", ""); err != nil {
			t.Fatalf("add project without scope should succeed: %v", err)
		}
	})
	//t.Run("test add project with team scope", func(t *testing.T) {
	//	if _, err := client.AddProject(context.Background(), "foo", user.ID); err != nil {
	//		t.Fatalf("add project with team scope should succeed: %v", err)
	//	}
	//})
}

func TestClose(t *testing.T) {
	conf, shutdown, err := makeTextile()
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	client, err := NewClient(conf.AddrApi)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test close", func(t *testing.T) {
		if err := client.Close(); err != nil {
			t.Fatalf("failed to close client: %v", err)
		}
	})
}

func makeTextile() (conf core.Config, shutdown func(), err error) {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return
	}

	apiPort, err := freeport.GetFreePort()
	if err != nil {
		return
	}
	gatewayPort, err := freeport.GetFreePort()
	if err != nil {
		return
	}
	threadsApiPort, err := freeport.GetFreePort()
	if err != nil {
		return
	}

	conf = core.Config{
		RepoPath:             dir,
		AddrApi:              util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", apiPort)),
		AddrThreadsHost:      util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrThreadsHostProxy: util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrThreadsApi:       util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", threadsApiPort)),
		AddrThreadsApiProxy:  util.MustParseAddr("/ip4/127.0.0.1/tcp/0"),
		AddrIpfsApi:          util.MustParseAddr("/ip4/127.0.0.1/tcp/5001"),

		AddrGateway:    util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gatewayPort)),
		AddrGatewayUrl: fmt.Sprintf("http://127.0.0.1:%d", gatewayPort),

		EmailFrom:   "test@email.textile.io",
		EmailDomain: "email.textile.io",
		EmailApiKey: "",

		SessionSecret: []byte(sessionSecret),

		Debug: true,
	}
	textile, err := core.NewTextile(conf)
	if err != nil {
		return
	}
	textile.Bootstrap()

	return conf, func() {
		time.Sleep(time.Second) // give threads a chance to finish work
		textile.Close()
		_ = os.RemoveAll(dir)
	}, nil
}
