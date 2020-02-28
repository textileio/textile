package client_test

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
	c "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/api/pb"
	"github.com/textileio/textile/core"
	"github.com/textileio/textile/util"
)

var (
	sessionSecret = uuid.New().String()
)

func TestClient_Login(t *testing.T) {
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

func TestClient_Switch(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	t.Run("test switch without token", func(t *testing.T) {
		if err := client.Switch(context.Background(), c.Auth{}); err == nil {
			t.Fatal("switch without token should fail")
		}
	})

	user := login(t, client, conf, "jon@doe.com")
	team, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test switch to team scope", func(t *testing.T) {
		if err := client.Switch(context.Background(), c.Auth{Token: user.SessionID, Scope: team.ID}); err != nil {
			t.Fatalf("switch to team scope should succeed: %v", err)
		}
	})

	t.Run("test switch to user scope", func(t *testing.T) {
		if err := client.Switch(context.Background(), c.Auth{Token: user.SessionID, Scope: user.ID}); err != nil {
			t.Fatalf("switch to user scope should succeed: %v", err)
		}
	})
}

func TestClient_Logout(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	t.Run("test logout without token", func(t *testing.T) {
		if err := client.Logout(context.Background(), c.Auth{}); err == nil {
			t.Fatal("logout without token should fail")
		}
	})

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test logout", func(t *testing.T) {
		if err := client.Logout(context.Background(), c.Auth{Token: user.SessionID}); err != nil {
			t.Fatalf("logout should succeed: %v", err)
		}
	})
}

func TestClient_Whoami(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	t.Run("test whoami without token", func(t *testing.T) {
		if _, err := client.Whoami(context.Background(), c.Auth{}); err == nil {
			t.Fatal("whoami without token should fail")
		}
	})

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test whoami", func(t *testing.T) {
		who, err := client.Whoami(context.Background(), c.Auth{Token: user.SessionID})
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

	team, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test whoami as team", func(t *testing.T) {
		who, err := client.Whoami(context.Background(), c.Auth{Token: user.SessionID, Scope: team.ID})
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

func TestClient_AddTeam(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	t.Run("test add team without token", func(t *testing.T) {
		if _, err := client.AddTeam(context.Background(), "foo", c.Auth{}); err == nil {
			t.Fatal("add team without token should fail")
		}
	})

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test add team", func(t *testing.T) {
		team, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("add team should succeed: %v", err)
		}
		if team.ID == "" {
			t.Fatal("got empty ID from add team")
		}
	})
}

func TestClient_GetTeam(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	team, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test get bad team", func(t *testing.T) {
		if _, err := client.GetTeam(context.Background(), "bad", c.Auth{Token: user.SessionID}); err == nil {
			t.Fatal("get bad team should fail")
		}
	})

	t.Run("test get team", func(t *testing.T) {
		team, err := client.GetTeam(context.Background(), team.ID, c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("get team should succeed: %v", err)
		}
		if team.ID == "" {
			t.Fatal("got empty ID from get team")
		}
	})
}

func TestClient_ListTeams(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test list empty teams", func(t *testing.T) {
		teams, err := client.ListTeams(context.Background(), c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("list teams should succeed: %v", err)
		}
		if len(teams.List) != 0 {
			t.Fatalf("got wrong team count from list teams, expected %d, got %d", 0, len(teams.List))
		}
	})

	if _, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AddTeam(context.Background(), "bar", c.Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}

	t.Run("test list teams", func(t *testing.T) {
		teams, err := client.ListTeams(context.Background(), c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("list teams should succeed: %v", err)
		}
		if len(teams.List) != 2 {
			t.Fatalf("got wrong team count from list teams, expected %d, got %d", 2, len(teams.List))
		}
	})
}

func TestClient_RemoveTeam(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	team, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test remove bad team", func(t *testing.T) {
		if err := client.RemoveTeam(context.Background(), "bad", c.Auth{Token: user.SessionID}); err == nil {
			t.Fatal("remove bad team should fail")
		}
	})

	user2 := login(t, client, conf, "jane@doe.com")

	t.Run("test remove team from wrong user", func(t *testing.T) {
		if err := client.RemoveTeam(context.Background(), team.ID, c.Auth{Token: user2.SessionID}); err == nil {
			t.Fatal("remove team from wrong user should fail")
		}
	})

	t.Run("test remove team", func(t *testing.T) {
		if err := client.RemoveTeam(context.Background(), team.ID, c.Auth{Token: user.SessionID}); err != nil {
			t.Fatalf("remove team should succeed: %v", err)
		}
	})
}

func TestClient_InviteToTeam(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	team, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test invite bad email to team", func(t *testing.T) {
		if _, err := client.InviteToTeam(context.Background(), team.ID, "jane",
			c.Auth{Token: user.SessionID}); err == nil {
			t.Fatal("invite bad email to team should fail")
		}
	})

	t.Run("test invite to team", func(t *testing.T) {
		res, err := client.InviteToTeam(context.Background(), team.ID, "jane@doe.com",
			c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("invite to team should succeed: %v", err)
		}
		if res.InviteID == "" {
			t.Fatal("got empty ID from invite to team")
		}
	})
}

func TestClient_LeaveTeam(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	team, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test leave team as owner", func(t *testing.T) {
		if err := client.LeaveTeam(context.Background(), team.ID, c.Auth{Token: user.SessionID}); err == nil {
			t.Fatal("leave team as owner should fail")
		}
	})

	user2 := login(t, client, conf, "jane@doe.com")

	t.Run("test leave team as non-member", func(t *testing.T) {
		if err := client.LeaveTeam(context.Background(), team.ID, c.Auth{Token: user2.SessionID}); err == nil {
			t.Fatal("leave team as non-member should fail")
		}
	})

	invite, err := client.InviteToTeam(context.Background(), team.ID, "jane@doe.com",
		c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	url := fmt.Sprintf("%s/consent/%s", conf.AddrGatewayUrl, invite.InviteID)
	if _, err := http.Get(url); err != nil {
		t.Fatal(err)
	}

	t.Run("test leave team", func(t *testing.T) {
		err := client.LeaveTeam(context.Background(), team.ID, c.Auth{Token: user2.SessionID})
		if err != nil {
			t.Fatalf("leave team should succeed: %v", err)
		}
	})
}

func TestClient_AddProject(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test add project without scope", func(t *testing.T) {
		proj, err := client.AddProject(context.Background(), "foo", c.Auth{Token: user.SessionID})
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

	team, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test add project with team scope", func(t *testing.T) {
		if _, err := client.AddProject(context.Background(), "bar",
			c.Auth{Token: user.SessionID, Scope: team.ID}); err != nil {
			t.Fatalf("add project with team scope should succeed: %v", err)
		}
	})
}

func TestClient_GetProject(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test get bad project", func(t *testing.T) {
		if _, err := client.GetProject(context.Background(), "bad",
			c.Auth{Token: user.SessionID}); err == nil {
			t.Fatal("get bad project should fail")
		}
	})

	team, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test get project from wrong scope", func(t *testing.T) {
		if _, err := client.GetProject(context.Background(), project.Name,
			c.Auth{Token: user.SessionID, Scope: team.ID}); err == nil {
			t.Fatal("get project from wrong scope should fail")
		}
	})

	t.Run("test get project", func(t *testing.T) {
		project, err := client.GetProject(context.Background(), project.Name, c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("get project should succeed: %v", err)
		}
		if project.ID == "" {
			t.Fatal("got empty ID from get project")
		}
	})
}

func TestClient_ListProjects(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test list empty projects", func(t *testing.T) {
		projects, err := client.ListProjects(context.Background(), c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("list projects should succeed: %v", err)
		}
		if len(projects.List) != 0 {
			t.Fatalf("got wrong project count from list projects, expected %d, got %d", 0,
				len(projects.List))
		}
	})

	if _, err := client.AddProject(context.Background(), "foo", c.Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AddProject(context.Background(), "bar", c.Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}

	team, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test list projects from wrong scope", func(t *testing.T) {
		projects, err := client.ListProjects(context.Background(), c.Auth{Token: user.SessionID, Scope: team.ID})
		if err != nil {
			t.Fatalf("list projects from wrong scope should succeed: %v", err)
		}
		if len(projects.List) != 0 {
			t.Fatalf("got wrong project count from list projects, expected %d, got %d", 0,
				len(projects.List))
		}
	})

	t.Run("test list projects", func(t *testing.T) {
		projects, err := client.ListProjects(context.Background(), c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("list projects should succeed: %v", err)
		}
		if len(projects.List) != 2 {
			t.Fatalf("got wrong project count from list projects, expected %d, got %d", 2,
				len(projects.List))
		}
	})
}

func TestClient_RemoveProject(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test remove bad project", func(t *testing.T) {
		if err := client.RemoveProject(context.Background(), "bad",
			c.Auth{Token: user.SessionID}); err == nil {
			t.Fatal("remove bad project should fail")
		}
	})

	user2 := login(t, client, conf, "jane@doe.com")

	t.Run("test remove project from wrong user", func(t *testing.T) {
		if err := client.RemoveProject(context.Background(), project.Name, c.Auth{Token: user2.SessionID}); err == nil {
			t.Fatal("remove project from wrong user should fail")
		}
	})

	team, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test remove project from wrong scope", func(t *testing.T) {
		if err := client.RemoveProject(context.Background(), project.Name,
			c.Auth{Token: user.SessionID, Scope: team.ID}); err == nil {
			t.Fatal("remove project from wrong scope should fail")
		}
	})

	t.Run("test remove project", func(t *testing.T) {
		if err := client.RemoveProject(context.Background(), project.Name, c.Auth{Token: user.SessionID}); err != nil {
			t.Fatalf("remove project should succeed: %v", err)
		}
	})
}

func TestClient_AddToken(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test add token", func(t *testing.T) {
		token, err := client.AddToken(context.Background(), project.Name, c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("add token should succeed: %v", err)
		}
		if token.ID == "" {
			t.Fatal("got empty ID from add token")
		}
	})
}

func TestClient_ListTokens(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test list empty tokens", func(t *testing.T) {
		tokens, err := client.ListTokens(context.Background(), project.Name, c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("list tokens should succeed: %v", err)
		}
		if len(tokens.List) != 0 {
			t.Fatalf("got wrong token count from list tokens, expected %d, got %d", 0,
				len(tokens.List))
		}
	})

	if _, err := client.AddToken(context.Background(), project.Name, c.Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.AddToken(context.Background(), project.Name, c.Auth{Token: user.SessionID}); err != nil {
		t.Fatal(err)
	}

	team, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test list tokens from wrong scope", func(t *testing.T) {
		if _, err := client.ListTokens(context.Background(), project.Name,
			c.Auth{Token: user.SessionID, Scope: team.ID}); err == nil {
			t.Fatal("list tokens from wrong scope should fail")
		}
	})

	t.Run("test list tokens", func(t *testing.T) {
		tokens, err := client.ListTokens(context.Background(), project.Name, c.Auth{Token: user.SessionID})
		if err != nil {
			t.Fatalf("list tokens should succeed: %v", err)
		}
		if len(tokens.List) != 2 {
			t.Fatalf("got wrong token count from list tokens, expected %d, got %d", 2,
				len(tokens.List))
		}
	})
}

func TestClient_RemoveToken(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	project, err := client.AddProject(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	token, err := client.AddToken(context.Background(), project.Name, c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test remove bad token", func(t *testing.T) {
		if err := client.RemoveToken(context.Background(), "bad",
			c.Auth{Token: user.SessionID}); err == nil {
			t.Fatal("remove bad token should fail")
		}
	})

	user2 := login(t, client, conf, "jane@doe.com")

	t.Run("test remove token from wrong user", func(t *testing.T) {
		if err := client.RemoveToken(context.Background(), token.ID, c.Auth{Token: user2.SessionID}); err == nil {
			t.Fatal("remove token from wrong user should fail")
		}
	})

	team, err := client.AddTeam(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test remove token from wrong scope", func(t *testing.T) {
		if err := client.RemoveToken(context.Background(), token.ID,
			c.Auth{Token: user.SessionID, Scope: team.ID}); err == nil {
			t.Fatal("remove token from wrong scope should fail")
		}
	})

	t.Run("test remove token", func(t *testing.T) {
		if err := client.RemoveToken(context.Background(), token.ID, c.Auth{Token: user.SessionID}); err != nil {
			t.Fatalf("remove token should succeed: %v", err)
		}
	})
}

func TestClient_RegisterUser(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")

	t.Run("test register user without token", func(t *testing.T) {
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

	project, err := client.AddProject(context.Background(), "foo", c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}
	token, err := client.AddToken(context.Background(), project.Name, c.Auth{Token: user.SessionID})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test register user", func(t *testing.T) {
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
			t.Logf("user id: %s", id)
		}
		session, ok := data["session_id"]
		if !ok {
			t.Fatalf("response body missing session id")
		} else {
			t.Logf("user session id: %s", session)
		}
		if res.StatusCode != http.StatusOK {
			t.Fatalf("expected status code 200, got %d", res.StatusCode)
		}
	})
}

func TestClient_ListBucketPath(t *testing.T) {
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
	conf, shutdown := makeTextile(t)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrApi)
	if err != nil {
		t.Fatal(err)
	}
	client, err := c.NewClient(target, nil)
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
	threadsServiceApiPort, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}
	threadsApiPort, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}

	conf = core.Config{
		RepoPath: dir,

		AddrApi:      util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", apiPort)),
		AddrApiProxy: util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),

		AddrThreadsHost: util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrThreadsServiceApi: util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d",
			threadsServiceApiPort)),
		AddrThreadsServiceApiProxy: util.MustParseAddr("/ip4/127.0.0.1/tcp/0"),
		AddrThreadsApi:             util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", threadsApiPort)),
		AddrThreadsApiProxy:        util.MustParseAddr("/ip4/127.0.0.1/tcp/0"),

		AddrIpfsApi: util.MustParseAddr("/ip4/127.0.0.1/tcp/5001"),

		AddrGatewayHost: util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gatewayPort)),
		AddrGatewayUrl:  fmt.Sprintf("http://127.0.0.1:%d", gatewayPort),

		EmailFrom:   "test@email.textile.io",
		EmailDomain: "email.textile.io",
		EmailApiKey: "",

		SessionSecret: sessionSecret,

		Debug: true,

		// @todo: When we have a lotus docker image,
		// add in AddrFilecoinApi option so filecoin is integrated into projects
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

func login(t *testing.T, client *c.Client, conf core.Config, email string) *pb.LoginReply {
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
