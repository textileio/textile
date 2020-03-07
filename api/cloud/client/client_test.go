package client_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api"
	c "github.com/textileio/textile/api/cloud/client"
	pb "github.com/textileio/textile/api/cloud/pb"
	"github.com/textileio/textile/core"
	"github.com/textileio/textile/util"
)

func TestClient_Login(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	assert.NotEmpty(t, user.Token)
}

func TestClient_Logout(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	t.Run("without token", func(t *testing.T) {
		err := client.Logout(context.Background(), api.Auth{})
		require.NotNil(t, err)
	})

	user := login(t, client, conf, "jon@doe.com")

	t.Run("with token", func(t *testing.T) {
		err := client.Logout(context.Background(), api.Auth{Token: user.Token})
		require.Nil(t, err)
	})
}

func TestClient_Whoami(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	t.Run("without token", func(t *testing.T) {
		_, err := client.Whoami(context.Background(), api.Auth{})
		require.NotNil(t, err)
	})

	user := login(t, client, conf, "jon@doe.com")

	t.Run("with token", func(t *testing.T) {
		who, err := client.Whoami(context.Background(), api.Auth{Token: user.Token})
		require.Nil(t, err)
		assert.Equal(t, who.ID, user.ID)
		assert.NotEmpty(t, who.Username)
		assert.Equal(t, who.Email, "jon@doe.com")
	})
}

func TestClient_AddOrg(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	t.Run("without token", func(t *testing.T) {
		_, err := client.AddOrg(context.Background(), "foo", api.Auth{})
		require.NotNil(t, err)
	})

	user := login(t, client, conf, "jon@doe.com")

	t.Run("with token", func(t *testing.T) {
		org, err := client.AddOrg(context.Background(), "foo", api.Auth{Token: user.Token})
		require.Nil(t, err)
		assert.NotEmpty(t, org.ID)
		assert.Equal(t, org.Name, "foo")
	})
}

func TestClient_GetOrg(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	org, err := client.AddOrg(context.Background(), "foo", api.Auth{Token: user.Token})
	require.Nil(t, err)

	t.Run("bad org", func(t *testing.T) {
		_, err := client.GetOrg(context.Background(), api.Auth{Token: user.Token, Org: "bad"})
		require.NotNil(t, err)
	})

	t.Run("good org", func(t *testing.T) {
		got, err := client.GetOrg(context.Background(), api.Auth{Token: user.Token, Org: org.Name})
		require.Nil(t, err)
		assert.Equal(t, got.ID, org.ID)
	})
}

func TestClient_ListOrgs(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")

	t.Run("empty", func(t *testing.T) {
		orgs, err := client.ListOrgs(context.Background(), api.Auth{Token: user.Token})
		require.Nil(t, err)
		assert.Empty(t, orgs.List)
	})

	_, err := client.AddOrg(context.Background(), "foo", api.Auth{Token: user.Token})
	require.Nil(t, err)
	_, err = client.AddOrg(context.Background(), "bar", api.Auth{Token: user.Token})
	require.Nil(t, err)

	t.Run("not empty", func(t *testing.T) {
		orgs, err := client.ListOrgs(context.Background(), api.Auth{Token: user.Token})
		require.Nil(t, err)
		assert.Equal(t, len(orgs.List), 2)
	})
}

func TestClient_RemoveOrg(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	org, err := client.AddOrg(context.Background(), "foo", api.Auth{Token: user.Token})
	require.Nil(t, err)

	t.Run("bad org", func(t *testing.T) {
		err := client.RemoveOrg(context.Background(), api.Auth{Token: user.Token, Org: "bad"})
		require.NotNil(t, err)
	})

	user2 := login(t, client, conf, "jane@doe.com")

	t.Run("bad session", func(t *testing.T) {
		err := client.RemoveOrg(context.Background(), api.Auth{Token: user2.Token, Org: org.Name})
		require.NotNil(t, err)
	})

	t.Run("good org", func(t *testing.T) {
		err := client.RemoveOrg(context.Background(), api.Auth{Token: user.Token, Org: org.Name})
		require.Nil(t, err)
		_, err = client.GetOrg(context.Background(), api.Auth{Token: user.Token, Org: org.Name})
		require.NotNil(t, err)
	})
}

func TestClient_InviteToOrg(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	org, err := client.AddOrg(context.Background(), "foo", api.Auth{Token: user.Token})
	require.Nil(t, err)

	t.Run("bad email", func(t *testing.T) {
		_, err := client.InviteToOrg(context.Background(), "jane", api.Auth{Token: user.Token, Org: org.Name})
		require.NotNil(t, err)
	})

	t.Run("good email", func(t *testing.T) {
		res, err := client.InviteToOrg(context.Background(), "jane@doe.com",
			api.Auth{Token: user.Token, Org: org.Name})
		require.Nil(t, err)
		assert.NotEmpty(t, res.Token)
	})
}

func TestClient_LeaveOrg(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := login(t, client, conf, "jon@doe.com")
	org, err := client.AddOrg(context.Background(), "foo", api.Auth{Token: user.Token})
	require.Nil(t, err)

	t.Run("as owner", func(t *testing.T) {
		err := client.LeaveOrg(context.Background(), api.Auth{Token: user.Token, Org: org.Name})
		require.NotNil(t, err)
	})

	user2 := login(t, client, conf, "jane@doe.com")

	t.Run("as non-member", func(t *testing.T) {
		err := client.LeaveOrg(context.Background(), api.Auth{Token: user2.Token, Org: org.Name})
		require.NotNil(t, err)
	})

	invite, err := client.InviteToOrg(context.Background(), "jane@doe.com",
		api.Auth{Token: user.Token, Org: org.Name})
	require.Nil(t, err)
	_, err = http.Get(fmt.Sprintf("%s/consent/%s", conf.AddrGatewayUrl, invite.Token))
	require.Nil(t, err)

	t.Run("as member", func(t *testing.T) {
		err := client.LeaveOrg(context.Background(), api.Auth{Token: user2.Token, Org: org.Name})
		require.Nil(t, err)
	})
}

func SkipTestClient_RegisterUser(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	login(t, client, conf, "jon@doe.com")

	t.Run("without token", func(t *testing.T) {
		url := fmt.Sprintf("%s/register", conf.AddrGatewayUrl)
		res, err := http.Post(url, "application/json", strings.NewReader("{}"))
		require.Nil(t, err)

		body, err := ioutil.ReadAll(res.Body)
		require.Nil(t, err)
		defer res.Body.Close()

		var data map[string]string
		err = json.Unmarshal(body, &data)
		require.Nil(t, err)

		if _, ok := data["error"]; !ok {
			t.Fatal("expected error in response body")
		}
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected status code 400, got %d", res.StatusCode)
		}
	})

	t.Run("with token", func(t *testing.T) {
		req, err := json.Marshal(&map[string]string{
			//"token":     token.ID,
			"device_id": uuid.New().String(),
		})
		require.Nil(t, err)
		url := fmt.Sprintf("%s/register", conf.AddrGatewayUrl)
		res, err := http.Post(url, "application/json", bytes.NewReader(req))
		require.Nil(t, err)

		body, err := ioutil.ReadAll(res.Body)
		require.Nil(t, err)
		defer res.Body.Close()

		var data map[string]string
		err = json.Unmarshal(body, &data)
		require.Nil(t, err)

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

func TestClose(t *testing.T) {
	t.Parallel()
	conf, shutdown := api.MakeTestTextile(t)
	defer shutdown()
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrApi)
	require.Nil(t, err)
	client, err := c.NewClient(target, nil)
	require.Nil(t, err)

	err = client.Close()
	require.Nil(t, err)
}

func setup(t *testing.T) (core.Config, *c.Client, func()) {
	conf, shutdown := api.MakeTestTextile(t)
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

func login(t *testing.T, client *c.Client, conf core.Config, email string) *pb.LoginReply {
	var err error
	var res *pb.LoginReply
	go func() {
		res, err = client.Login(context.Background(), util.MakeToken(12), email)
		require.Nil(t, err)
	}()

	// Ensure login request has processed
	time.Sleep(time.Second)
	url := fmt.Sprintf("%s/confirm/%s", conf.AddrGatewayUrl, api.TestSessionSecret)
	_, err = http.Get(url)
	require.Nil(t, err)

	// Ensure login response has been received
	time.Sleep(time.Second)
	require.NotNil(t, res)
	require.NotEmpty(t, res.Token)
	return res
}
