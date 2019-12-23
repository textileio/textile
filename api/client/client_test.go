package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/textileio/textile/api/pb"
	"github.com/textileio/textile/core"
	"github.com/textileio/textile/util"
)

var (
	addrApi        = util.MustParseAddr("/ip4/127.0.0.1/tcp/3006")
	addrGateway    = util.MustParseAddr("/ip4/127.0.0.1/tcp/8006")
	addrGatewayUrl = "http://127.0.0.1:8006"
	sessionSecret  = "test_runner"
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
		var res *pb.LoginReply
		go func() {
			res, err = client.Login(context.Background(), "jon@doe.com")
			if err != nil {
				t.Fatal(err)
			}
		}()

		// Ensure login request has processed
		time.Sleep(time.Second)
		verificationURL := fmt.Sprintf("%s/verify/%s", addrGatewayUrl, sessionSecret)
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
	shutdown, err := makeTextile()
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	client, err := NewClient(addrApi)
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
	shutdown, err := makeTextile()
	if err != nil {
		t.Fatal(err)
	}
	defer shutdown()
	client, err := NewClient(addrApi)
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
		AddrThreadsHost:      util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrThreadsHostProxy: util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		// @todo: Currently, this can't be port zero because the client would not
		// know the randomly chosen listen port.
		AddrThreadsApi:      util.MustParseAddr("/ip4/127.0.0.1/tcp/6006"),
		AddrThreadsApiProxy: util.MustParseAddr("/ip4/127.0.0.1/tcp/0"),
		AddrIpfsApi:         util.MustParseAddr("/ip4/127.0.0.1/tcp/5001"),

		AddrGateway:    addrGateway,
		AddrGatewayUrl: addrGatewayUrl,

		EmailFrom:   "test@email.textile.io",
		EmailDomain: "email.textile.io",
		EmailApiKey: "",

		SessionSecret: []byte(sessionSecret),

		Debug: true,
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
