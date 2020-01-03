package client

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
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
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	t.Run("test login", func(t *testing.T) {
		user := login(t, client, conf, "jon@doe.com")
		if user.Token == "" {
			t.Fatal("got empty token from login")
		}
	})
}

func TestSwitch(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	t.Run("test switch without token", func(t *testing.T) {
		if err := client.Switch(context.Background(), Auth{}); err == nil {
			t.Fatal("switch without token should fail")
		}
	})

	user := login(t, client, conf, "jon@doe.com")
	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.Token})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test switch to team scope", func(t *testing.T) {
		if err := client.Switch(context.Background(), Auth{Token: user.Token, Scope: team.ID}); err != nil {
			t.Fatalf("switch to team scope should succeed: %v", err)
		}
	})

	t.Run("test switch to user scope", func(t *testing.T) {
		if err := client.Switch(context.Background(), Auth{Token: user.Token, Scope: user.ID}); err != nil {
			t.Fatalf("switch to user scope should succeed: %v", err)
		}
	})
}

func TestLogout(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	t.Run("test logout without token", func(t *testing.T) {
		if err := client.Logout(context.Background(), Auth{}); err == nil {
			t.Fatal("logout without token should fail")
		}
	})

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test logout", func(t *testing.T) {
		if err := client.Logout(context.Background(), Auth{Token: user.Token}); err != nil {
			t.Fatalf("logout should succeed: %v", err)
		}
	})
}

func TestWhoami(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	t.Run("test whoami without token", func(t *testing.T) {
		if _, err := client.Whoami(context.Background(), Auth{}); err == nil {
			t.Fatal("whoami without token should fail")
		}
	})

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test whoami", func(t *testing.T) {
		who, err := client.Whoami(context.Background(), Auth{Token: user.Token})
		if err != nil {
			t.Fatalf("whoami should succeed: %v", err)
		}
		if who.ID != user.ID {
			t.Fatal("got bad ID from whoami")
		}
		if who.Email != "jon@doe.com" {
			t.Fatal("got bad name from whoami")
		}
	})

	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.Token})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test whoami as team", func(t *testing.T) {
		who, err := client.Whoami(context.Background(), Auth{Token: user.Token, Scope: team.ID})
		if err != nil {
			t.Fatalf("whoami as team should succeed: %v", err)
		}
		if who.TeamID != team.ID {
			t.Fatal("got bad ID from whoami")
		}
		if who.TeamName != "foo" {
			t.Fatal("got bad name from whoami")
		}
	})
}

func TestAddTeam(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	t.Run("test add team without token", func(t *testing.T) {
		if _, err := client.AddTeam(context.Background(), "foo", Auth{}); err == nil {
			t.Fatal("add team without token should fail")
		}
	})

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test add team", func(t *testing.T) {
		team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.Token})
		if err != nil {
			t.Fatalf("add team should succeed: %v", err)
		}
		if team.ID == "" {
			t.Fatal("got empty ID from add team")
		}
	})
}

func TestGetTeam(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.Token})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test get bad team", func(t *testing.T) {
		if _, err := client.GetTeam(context.Background(), "bad", Auth{Token: user.Token}); err == nil {
			t.Fatal("get bad team should fail")
		}
	})

	t.Run("test get team", func(t *testing.T) {
		team, err := client.GetTeam(context.Background(), team.ID, Auth{Token: user.Token})
		if err != nil {
			t.Fatalf("get team should succeed: %v", err)
		}
		if team.ID == "" {
			t.Fatal("got empty ID from get team")
		}
	})
}

func TestListTeams(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test list empty teams", func(t *testing.T) {
		teams, err := client.ListTeams(context.Background(), Auth{Token: user.Token})
		if err != nil {
			t.Fatalf("list teams should succeed: %v", err)
		}
		if len(teams.List) != 0 {
			t.Fatalf("got wrong team count from list teams, expected %d, got %d", 0, len(teams.List))
		}
	})

	if _, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.Token}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AddTeam(context.Background(), "bar", Auth{Token: user.Token}); err != nil {
		t.Fatal(err)
	}

	t.Run("test list teams", func(t *testing.T) {
		teams, err := client.ListTeams(context.Background(), Auth{Token: user.Token})
		if err != nil {
			t.Fatalf("list teams should succeed: %v", err)
		}
		if len(teams.List) != 2 {
			t.Fatalf("got wrong team count from list teams, expected %d, got %d", 2, len(teams.List))
		}
	})
}

func TestRemoveTeam(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.Token})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test remove bad team", func(t *testing.T) {
		if err := client.RemoveTeam(context.Background(), "bad", Auth{Token: user.Token}); err == nil {
			t.Fatal("remove bad team should fail")
		}
	})

	user2 := login(t, client, conf, "jane@doe.com")

	t.Run("test remove team from wrong user", func(t *testing.T) {
		if err := client.RemoveTeam(context.Background(), team.ID, Auth{Token: user2.Token}); err == nil {
			t.Fatal("remove team from wrong user should fail")
		}
	})

	t.Run("test remove team", func(t *testing.T) {
		if err := client.RemoveTeam(context.Background(), team.ID, Auth{Token: user.Token}); err != nil {
			t.Fatalf("remove team should succeed: %v", err)
		}
	})
}

func TestInviteToTeam(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.Token})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test invite bad email to team", func(t *testing.T) {
		if _, err := client.InviteToTeam(context.Background(), team.ID, "jane",
			Auth{Token: user.Token}); err == nil {
			t.Fatal("invite bad email to team should fail")
		}
	})

	t.Run("test invite to team", func(t *testing.T) {
		res, err := client.InviteToTeam(context.Background(), team.ID, "jane@doe.com",
			Auth{Token: user.Token})
		if err != nil {
			t.Fatalf("invite to team should succeed: %v", err)
		}
		if res.InviteID == "" {
			t.Fatal("got empty ID from invite to team")
		}
	})
}

func TestLeaveTeam(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.Token})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test leave team as owner", func(t *testing.T) {
		if err := client.LeaveTeam(context.Background(), team.ID, Auth{Token: user.Token}); err == nil {
			t.Fatal("leave team as owner should fail")
		}
	})

	user2 := login(t, client, conf, "jane@doe.com")

	t.Run("test leave team as non-member", func(t *testing.T) {
		if err := client.LeaveTeam(context.Background(), team.ID, Auth{Token: user2.Token}); err == nil {
			t.Fatal("leave team as non-member should fail")
		}
	})

	invite, err := client.InviteToTeam(context.Background(), team.ID, "jane@doe.com",
		Auth{Token: user.Token})
	if err != nil {
		t.Fatal(err)
	}
	url := fmt.Sprintf("%s/consent/%s", conf.AddrGatewayUrl, invite.InviteID)
	if _, err := http.Get(url); err != nil {
		t.Fatalf("failed to reach gateway: %v", err)
	}

	t.Run("test leave team", func(t *testing.T) {
		err := client.LeaveTeam(context.Background(), team.ID, Auth{Token: user2.Token})
		if err != nil {
			t.Fatalf("leave team should succeed: %v", err)
		}
	})
}

func TestAddProject(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test add project without scope", func(t *testing.T) {
		proj, err := client.AddProject(context.Background(), "foo", Auth{Token: user.Token})
		if err != nil {
			t.Fatalf("add project without scope should succeed: %v", err)
		}
		if proj.ID == "" {
			t.Fatal("got empty ID from add project")
		}
		if proj.StoreID == "" {
			t.Fatal("got empty store ID from add project")
		}
	})

	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.Token})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test add project with team scope", func(t *testing.T) {
		if _, err := client.AddProject(context.Background(), "foo",
			Auth{Token: user.Token, Scope: team.ID}); err != nil {
			t.Fatalf("add project with team scope should succeed: %v", err)
		}
	})
}

func TestGetProject(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", Auth{Token: user.Token})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test get bad project", func(t *testing.T) {
		if _, err := client.GetProject(context.Background(), "bad", Auth{Token: user.Token}); err == nil {
			t.Fatal("get bad project should fail")
		}
	})

	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.Token})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test get project from wrong scope", func(t *testing.T) {
		if _, err := client.GetProject(context.Background(), project.ID,
			Auth{Token: user.Token, Scope: team.ID}); err == nil {
			t.Fatal("get project from wrong scope should fail")
		}
	})

	t.Run("test get project", func(t *testing.T) {
		project, err := client.GetProject(context.Background(), project.ID, Auth{Token: user.Token})
		if err != nil {
			t.Fatalf("get project should succeed: %v", err)
		}
		if project.ID == "" {
			t.Fatal("got empty ID from get project")
		}
	})
}

func TestListProjects(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test list empty projects", func(t *testing.T) {
		projects, err := client.ListProjects(context.Background(), Auth{Token: user.Token})
		if err != nil {
			t.Fatalf("list projects should succeed: %v", err)
		}
		if len(projects.List) != 0 {
			t.Fatalf("got wrong project count from list projects, expected %d, got %d", 0, len(projects.List))
		}
	})

	if _, err := client.AddProject(context.Background(), "foo", Auth{Token: user.Token}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AddProject(context.Background(), "bar", Auth{Token: user.Token}); err != nil {
		t.Fatal(err)
	}

	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.Token})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test list projects from wrong scope", func(t *testing.T) {
		projects, err := client.ListProjects(context.Background(), Auth{Token: user.Token, Scope: team.ID})
		if err != nil {
			t.Fatalf("list projects from wrong scope should succeed: %v", err)
		}
		if len(projects.List) != 0 {
			t.Fatalf("got wrong project count from list projects, expected %d, got %d", 0, len(projects.List))
		}
	})

	t.Run("test list projects", func(t *testing.T) {
		projects, err := client.ListProjects(context.Background(), Auth{Token: user.Token})
		if err != nil {
			t.Fatalf("list projects should succeed: %v", err)
		}
		if len(projects.List) != 2 {
			t.Fatalf("got wrong project count from list projects, expected %d, got %d", 2, len(projects.List))
		}
	})
}

func TestRemoveProject(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", Auth{Token: user.Token})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test remove bad project", func(t *testing.T) {
		if err := client.RemoveProject(context.Background(), "bad", Auth{Token: user.Token}); err == nil {
			t.Fatal("remove bad project should fail")
		}
	})

	user2 := login(t, client, conf, "jane@doe.com")

	t.Run("test remove project from wrong user", func(t *testing.T) {
		if err := client.RemoveProject(context.Background(), project.ID, Auth{Token: user2.Token}); err == nil {
			t.Fatal("remove project from wrong user should fail")
		}
	})

	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.Token})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test remove project from wrong scope", func(t *testing.T) {
		if err := client.RemoveProject(context.Background(), project.ID,
			Auth{Token: user.Token, Scope: team.ID}); err == nil {
			t.Fatal("remove project from wrong scope should fail")
		}
	})

	t.Run("test remove project", func(t *testing.T) {
		if err := client.RemoveProject(context.Background(), project.ID, Auth{Token: user.Token}); err != nil {
			t.Fatalf("remove project should succeed: %v", err)
		}
	})
}

func TestClose(t *testing.T) {
	t.Parallel()
	conf, shutdown := makeTextile(t)
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

func setup(t *testing.T) (core.Config, *Client, func()) {
	conf, shutdown := makeTextile(t)
	client, err := NewClient(conf.AddrApi)
	if err != nil {
		t.Fatal(err)
	}

	return conf, client, func() {
		shutdown()
		client.Close()
	}
}

func makeTextile(t *testing.T) (conf core.Config, shutdown func()) {
	time.Sleep(time.Second * time.Duration(rand.Intn(5)))

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	apiPort, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}
	gatewayPort, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}
	threadsApiPort, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}

	conf = core.Config{
		RepoPath:             dir,
		AddrApi:              util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", apiPort)),
		AddrThreadsHost:      util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrThreadsHostProxy: util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrThreadsApi:       util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", threadsApiPort)),
		AddrThreadsApiProxy:  util.MustParseAddr("/ip4/127.0.0.1/tcp/0"),
		AddrIpfsApi:          util.MustParseAddr("/ip4/127.0.0.1/tcp/5001"),
		AddrFilecoinServer:   util.MustParseAddr("/ip4/127.0.0.1/tcp/5002"),

		AddrGateway:    util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gatewayPort)),
		AddrGatewayUrl: fmt.Sprintf("http://127.0.0.1:%d", gatewayPort),

		EmailFrom:   "test@email.textile.io",
		EmailDomain: "email.textile.io",
		EmailApiKey: "",

		SessionSecret: []byte(sessionSecret),

		Debug: true,
	}
	textile, err := core.NewTextile(context.Background(), conf)
	if err != nil {
		t.Fatal(err)
	}
	textile.Bootstrap()

	return conf, func() {
		time.Sleep(time.Second) // give threads a chance to finish work
		textile.Close()
		_ = os.RemoveAll(dir)
	}
}

func login(t *testing.T, client *Client, conf core.Config, email string) *pb.LoginReply {
	var err error
	var res *pb.LoginReply
	go func() {
		res, err = client.Login(context.Background(), email)
		if err != nil {
			t.Fatal(err)
		}
	}()

	// Ensure login request has processed
	time.Sleep(time.Second)
	url := fmt.Sprintf("%s/confirm/%s", conf.AddrGatewayUrl, sessionSecret)
	if _, err := http.Get(url); err != nil {
		t.Fatalf("failed to reach gateway: %v", err)
	}

	// Ensure login response has been received
	time.Sleep(time.Second)
	if res == nil || res.Token == "" {
		t.Fatal("got empty token from login")
	}
	return res
}
