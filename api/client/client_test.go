package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/phayes/freeport"
	tutil "github.com/textileio/go-threads/util"
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
		if user.SessionID == "" {
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
	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test switch to team scope", func(t *testing.T) {
		if err := client.Switch(context.Background(), Auth{Token: user.SessionID, Scope: team.ID}); err != nil {
			t.Fatalf("switch to team scope should succeed: %v", err)
		}
	})

	t.Run("test switch to user scope", func(t *testing.T) {
		if err := client.Switch(context.Background(), Auth{Token: user.SessionID, Scope: user.ID}); err != nil {
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
		if err := client.Logout(context.Background(), Auth{Token: user.SessionID}); err != nil {
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
		who, err := client.Whoami(context.Background(), Auth{Token: user.SessionID})
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

	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test whoami as team", func(t *testing.T) {
		who, err := client.Whoami(context.Background(), Auth{Token: user.SessionID, Scope: team.ID})
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
		team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID})
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
	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test get bad team", func(t *testing.T) {
		if _, err := client.GetTeam(context.Background(), "bad", Auth{Token: user.SessionID}); err == nil {
			t.Fatal("get bad team should fail")
		}
	})

	t.Run("test get team", func(t *testing.T) {
		team, err := client.GetTeam(context.Background(), team.ID, Auth{Token: user.SessionID})
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
		teams, err := client.ListTeams(context.Background(), Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("list teams should succeed: %v", err)
		}
		if len(teams.List) != 0 {
			t.Fatalf("got wrong team count from list teams, expected %d, got %d", 0, len(teams.List))
		}
	})

	if _, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AddTeam(context.Background(), "bar", Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}

	t.Run("test list teams", func(t *testing.T) {
		teams, err := client.ListTeams(context.Background(), Auth{Token: user.SessionID})
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
	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test remove bad team", func(t *testing.T) {
		if err := client.RemoveTeam(context.Background(), "bad", Auth{Token: user.SessionID}); err == nil {
			t.Fatal("remove bad team should fail")
		}
	})

	user2 := login(t, client, conf, "jane@doe.com")

	t.Run("test remove team from wrong user", func(t *testing.T) {
		if err := client.RemoveTeam(context.Background(), team.ID, Auth{Token: user2.SessionID}); err == nil {
			t.Fatal("remove team from wrong user should fail")
		}
	})

	t.Run("test remove team", func(t *testing.T) {
		if err := client.RemoveTeam(context.Background(), team.ID, Auth{Token: user.SessionID}); err != nil {
			t.Fatalf("remove team should succeed: %v", err)
		}
	})
}

func TestInviteToTeam(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test invite bad email to team", func(t *testing.T) {
		if _, err := client.InviteToTeam(context.Background(), team.ID, "jane",
			Auth{Token: user.SessionID}); err == nil {
			t.Fatal("invite bad email to team should fail")
		}
	})

	t.Run("test invite to team", func(t *testing.T) {
		res, err := client.InviteToTeam(context.Background(), team.ID, "jane@doe.com",
			Auth{Token: user.SessionID})
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
	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test leave team as owner", func(t *testing.T) {
		if err := client.LeaveTeam(context.Background(), team.ID, Auth{Token: user.SessionID}); err == nil {
			t.Fatal("leave team as owner should fail")
		}
	})

	user2 := login(t, client, conf, "jane@doe.com")

	t.Run("test leave team as non-member", func(t *testing.T) {
		if err := client.LeaveTeam(context.Background(), team.ID, Auth{Token: user2.SessionID}); err == nil {
			t.Fatal("leave team as non-member should fail")
		}
	})

	invite, err := client.InviteToTeam(context.Background(), team.ID, "jane@doe.com",
		Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	url := fmt.Sprintf("%s/consent/%s", conf.AddrGatewayUrl, invite.InviteID)
	if _, err := http.Get(url); err != nil {
		t.Fatal(err)
	}

	t.Run("test leave team", func(t *testing.T) {
		err := client.LeaveTeam(context.Background(), team.ID, Auth{Token: user2.SessionID})
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
		proj, err := client.AddProject(context.Background(), "foo", Auth{Token: user.SessionID})
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

	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test add project with team scope", func(t *testing.T) {
		if _, err := client.AddProject(context.Background(), "foo",
			Auth{Token: user.SessionID, Scope: team.ID}); err != nil {
			t.Fatalf("add project with team scope should succeed: %v", err)
		}
	})
}

func TestGetProject(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test get bad project", func(t *testing.T) {
		if _, err := client.GetProject(context.Background(), "bad", Auth{Token: user.SessionID}); err == nil {
			t.Fatal("get bad project should fail")
		}
	})

	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test get project from wrong scope", func(t *testing.T) {
		if _, err := client.GetProject(context.Background(), project.ID,
			Auth{Token: user.SessionID, Scope: team.ID}); err == nil {
			t.Fatal("get project from wrong scope should fail")
		}
	})

	t.Run("test get project", func(t *testing.T) {
		project, err := client.GetProject(context.Background(), project.ID, Auth{Token: user.SessionID})
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
		projects, err := client.ListProjects(context.Background(), Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("list projects should succeed: %v", err)
		}
		if len(projects.List) != 0 {
			t.Fatalf("got wrong project count from list projects, expected %d, got %d", 0,
				len(projects.List))
		}
	})

	if _, err := client.AddProject(context.Background(), "foo", Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AddProject(context.Background(), "bar", Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}

	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test list projects from wrong scope", func(t *testing.T) {
		projects, err := client.ListProjects(context.Background(), Auth{Token: user.SessionID, Scope: team.ID})
		if err != nil {
			t.Fatalf("list projects from wrong scope should succeed: %v", err)
		}
		if len(projects.List) != 0 {
			t.Fatalf("got wrong project count from list projects, expected %d, got %d", 0,
				len(projects.List))
		}
	})

	t.Run("test list projects", func(t *testing.T) {
		projects, err := client.ListProjects(context.Background(), Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("list projects should succeed: %v", err)
		}
		if len(projects.List) != 2 {
			t.Fatalf("got wrong project count from list projects, expected %d, got %d", 2,
				len(projects.List))
		}
	})
}

func TestRemoveProject(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test remove bad project", func(t *testing.T) {
		if err := client.RemoveProject(context.Background(), "bad", Auth{Token: user.SessionID}); err == nil {
			t.Fatal("remove bad project should fail")
		}
	})

	user2 := login(t, client, conf, "jane@doe.com")

	t.Run("test remove project from wrong user", func(t *testing.T) {
		if err := client.RemoveProject(context.Background(), project.ID, Auth{Token: user2.SessionID}); err == nil {
			t.Fatal("remove project from wrong user should fail")
		}
	})

	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test remove project from wrong scope", func(t *testing.T) {
		if err := client.RemoveProject(context.Background(), project.ID,
			Auth{Token: user.SessionID, Scope: team.ID}); err == nil {
			t.Fatal("remove project from wrong scope should fail")
		}
	})

	t.Run("test remove project", func(t *testing.T) {
		if err := client.RemoveProject(context.Background(), project.ID, Auth{Token: user.SessionID}); err != nil {
			t.Fatalf("remove project should succeed: %v", err)
		}
	})
}

func TestAddAppToken(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test add app token", func(t *testing.T) {
		token, err := client.AddAppToken(context.Background(), project.ID, Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("add app token should succeed: %v", err)
		}
		if token.ID == "" {
			t.Fatal("got empty ID from add token")
		}
	})
}

func TestListAppTokens(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test list empty app tokens", func(t *testing.T) {
		tokens, err := client.ListAppTokens(context.Background(), project.ID, Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("list app tokens should succeed: %v", err)
		}
		if len(tokens.List) != 0 {
			t.Fatalf("got wrong app token count from list app tokens, expected %d, got %d", 0,
				len(tokens.List))
		}
	})

	if _, err := client.AddAppToken(context.Background(), project.ID, Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AddAppToken(context.Background(), project.ID, Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}

	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test list app tokens from wrong scope", func(t *testing.T) {
		if _, err := client.ListAppTokens(context.Background(), project.ID,
			Auth{Token: user.SessionID, Scope: team.ID}); err == nil {
			t.Fatal("list app tokens from wrong scope should fail")
		}
	})

	t.Run("test list app tokens", func(t *testing.T) {
		tokens, err := client.ListAppTokens(context.Background(), project.ID, Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("list app tokens should succeed: %v", err)
		}
		if len(tokens.List) != 2 {
			t.Fatalf("got wrong app token count from list app tokens, expected %d, got %d", 2,
				len(tokens.List))
		}
	})
}

func TestRemoveAppToken(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	token, err := client.AddAppToken(context.Background(), project.ID, Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test remove bad app token", func(t *testing.T) {
		if err := client.RemoveAppToken(context.Background(), "bad", Auth{Token: user.SessionID}); err == nil {
			t.Fatal("remove bad app token should fail")
		}
	})

	user2 := login(t, client, conf, "jane@doe.com")

	t.Run("test remove app token from wrong user", func(t *testing.T) {
		if err := client.RemoveAppToken(context.Background(), token.ID, Auth{Token: user2.SessionID}); err == nil {
			t.Fatal("remove app token from wrong user should fail")
		}
	})

	team, err := client.AddTeam(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test remove app token from wrong scope", func(t *testing.T) {
		if err := client.RemoveAppToken(context.Background(), token.ID,
			Auth{Token: user.SessionID, Scope: team.ID}); err == nil {
			t.Fatal("remove app token from wrong scope should fail")
		}
	})

	t.Run("test remove app token", func(t *testing.T) {
		if err := client.RemoveAppToken(context.Background(), token.ID, Auth{Token: user.SessionID}); err != nil {
			t.Fatalf("remove app token should succeed: %v", err)
		}
	})
}

func TestRegisterAppUser(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test register app user without token", func(t *testing.T) {
		url := fmt.Sprintf("%s/register", conf.AddrGatewayUrl)
		res, err := http.Post(url, "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()

		var data map[string]string
		if err := json.Unmarshal(body, &data); err != nil {
			t.Fatal(err)
		}
		if _, ok := data["error"]; !ok {
			t.Fatal("expected error in response body")
		}
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status code 400, got %d", res.StatusCode)
		}
	})

	project, err := client.AddProject(context.Background(), "foo", Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	token, err := client.AddAppToken(context.Background(), project.ID, Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test register app user", func(t *testing.T) {
		req, err := json.Marshal(&map[string]string{
			"token":     token.ID,
			"device_id": uuid.New().String(),
		})
		if err != nil {
			t.Fatal(err)
		}
		url := fmt.Sprintf("%s/register", conf.AddrGatewayUrl)
		res, err := http.Post(url, "application/json", bytes.NewReader(req))
		if err != nil {
			t.Fatal(err)
		}

		body, err := ioutil.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()

		var data map[string]string
		if err := json.Unmarshal(body, &data); err != nil {
			t.Fatal(err)
		}
		if e, ok := data["error"]; ok {
			t.Fatalf("got error in response body: %s", e)
		}
		if id, ok := data["id"]; !ok {
			t.Fatalf("response body missing id")
		} else {
			t.Logf("app user id: %s", id)
		}
		if session, ok := data["session_id"]; !ok {
			t.Fatalf("response body missing session id")
		} else {
			t.Logf("app user session id: %s", session)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected status code 200, got %d", res.StatusCode)
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
	client, err := NewClient(target, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test close", func(t *testing.T) {
		if err := client.Close(); err != nil {
			t.Fatal(err)
		}
	})
}

func setup(t *testing.T) (core.Config, *Client, func()) {
	conf, shutdown := makeTextile(t)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrApi)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClient(target, nil)
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

		AddrGatewayHost: util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gatewayPort)),
		AddrGatewayUrl:  fmt.Sprintf("http://127.0.0.1:%d", gatewayPort),

		EmailFrom:   "test@email.textile.io",
		EmailDomain: "email.textile.io",
		EmailApiKey: "",

		SessionSecret: []byte(sessionSecret),

		Debug: true,
		// ToDo: add in AddrFilecoinApi option so filecoin is integrated into projects
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
		t.Fatal(err)
	}

	// Ensure login response has been received
	time.Sleep(time.Second)
	if res == nil || res.SessionID == "" {
		t.Fatal("got empty token from login")
	}
	return res
}
