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

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/apitest"
	c "github.com/textileio/textile/api/cloud/client"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/core"
	"google.golang.org/grpc"
)

func TestClient_Login(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := apitest.Login(t, client, conf, apitest.NewEmail())
	assert.NotEmpty(t, user.Token)
}

func TestClient_Logout(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()
	ctx := context.Background()

	t.Run("without token", func(t *testing.T) {
		err := client.Logout(ctx)
		require.NotNil(t, err)
	})

	user := apitest.Login(t, client, conf, apitest.NewEmail())

	t.Run("with token", func(t *testing.T) {
		err := client.Logout(common.NewDevTokenContext(ctx, user.Token))
		require.Nil(t, err)
	})
}

func TestClient_Whoami(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()
	ctx := context.Background()

	t.Run("without token", func(t *testing.T) {
		_, err := client.Whoami(ctx)
		require.NotNil(t, err)
	})

	email := apitest.NewEmail()
	user := apitest.Login(t, client, conf, email)

	t.Run("with token", func(t *testing.T) {
		who, err := client.Whoami(common.NewDevTokenContext(ctx, user.Token))
		require.Nil(t, err)
		assert.Equal(t, who.ID, user.ID)
		assert.NotEmpty(t, who.Username)
		assert.Equal(t, who.Email, email)
	})
}

func TestClient_AddOrg(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()
	ctx := context.Background()

	name := apitest.NewName()

	t.Run("without token", func(t *testing.T) {
		_, err := client.AddOrg(ctx, name)
		require.NotNil(t, err)
	})

	user := apitest.Login(t, client, conf, apitest.NewEmail())

	t.Run("with token", func(t *testing.T) {
		org, err := client.AddOrg(common.NewDevTokenContext(ctx, user.Token), name)
		require.Nil(t, err)
		assert.NotEmpty(t, org.ID)
		assert.Equal(t, org.Name, name)
	})
}

func TestClient_GetOrg(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	name := apitest.NewName()
	user := apitest.Login(t, client, conf, apitest.NewEmail())
	ctx := common.NewDevTokenContext(context.Background(), user.Token)
	org, err := client.AddOrg(ctx, name)
	require.Nil(t, err)

	t.Run("bad org", func(t *testing.T) {
		_, err := client.GetOrg(common.NewOrgContext(ctx, "bad"))
		require.NotNil(t, err)
	})

	t.Run("good org", func(t *testing.T) {
		got, err := client.GetOrg(common.NewOrgContext(ctx, org.Name))
		require.Nil(t, err)
		assert.Equal(t, got.ID, org.ID)
	})
}

func TestClient_ListOrgs(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	user := apitest.Login(t, client, conf, apitest.NewEmail())
	ctx := common.NewDevTokenContext(context.Background(), user.Token)

	t.Run("empty", func(t *testing.T) {
		orgs, err := client.ListOrgs(ctx)
		require.Nil(t, err)
		assert.Empty(t, orgs.List)
	})

	name1 := uuid.New().String()
	_, err := client.AddOrg(ctx, name1)
	require.Nil(t, err)
	name2 := uuid.New().String()
	_, err = client.AddOrg(ctx, name2)
	require.Nil(t, err)

	t.Run("not empty", func(t *testing.T) {
		orgs, err := client.ListOrgs(ctx)
		require.Nil(t, err)
		assert.Equal(t, len(orgs.List), 2)
	})
}

func TestClient_RemoveOrg(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	name := apitest.NewName()
	user := apitest.Login(t, client, conf, apitest.NewEmail())
	ctx := common.NewDevTokenContext(context.Background(), user.Token)
	org, err := client.AddOrg(ctx, name)
	require.Nil(t, err)

	t.Run("bad org", func(t *testing.T) {
		err := client.RemoveOrg(common.NewOrgContext(ctx, "bad"))
		require.NotNil(t, err)
	})

	user2 := apitest.Login(t, client, conf, apitest.NewEmail())
	ctx2 := common.NewDevTokenContext(context.Background(), user2.Token)

	t.Run("bad session", func(t *testing.T) {
		err := client.RemoveOrg(common.NewOrgContext(ctx2, org.Name))
		require.NotNil(t, err)
	})

	t.Run("good org", func(t *testing.T) {
		octx := common.NewOrgContext(ctx, org.Name)
		err := client.RemoveOrg(octx)
		require.Nil(t, err)
		_, err = client.GetOrg(octx)
		require.NotNil(t, err)
	})
}

func TestClient_InviteToOrg(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	name := apitest.NewName()
	user := apitest.Login(t, client, conf, apitest.NewEmail())
	ctx := common.NewDevTokenContext(context.Background(), user.Token)
	org, err := client.AddOrg(ctx, name)
	require.Nil(t, err)
	ctx = common.NewOrgContext(ctx, org.Name)

	t.Run("bad email", func(t *testing.T) {
		_, err := client.InviteToOrg(ctx, "jane")
		require.NotNil(t, err)
	})

	t.Run("good email", func(t *testing.T) {
		res, err := client.InviteToOrg(ctx, apitest.NewEmail())
		require.Nil(t, err)
		assert.NotEmpty(t, res.Token)
	})
}

func TestClient_LeaveOrg(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	name := apitest.NewName()
	user := apitest.Login(t, client, conf, apitest.NewEmail())
	ctx := common.NewDevTokenContext(context.Background(), user.Token)
	org, err := client.AddOrg(ctx, name)
	require.Nil(t, err)
	ctx = common.NewOrgContext(ctx, org.Name)

	t.Run("as owner", func(t *testing.T) {
		err := client.LeaveOrg(ctx)
		require.NotNil(t, err)
	})

	user2Email := apitest.NewEmail()
	user2 := apitest.Login(t, client, conf, user2Email)
	ctx2 := common.NewDevTokenContext(ctx, user2.Token)

	t.Run("as non-member", func(t *testing.T) {
		err := client.LeaveOrg(ctx2)
		require.NotNil(t, err)
	})

	invite, err := client.InviteToOrg(ctx, user2Email)
	require.Nil(t, err)
	_, err = http.Get(fmt.Sprintf("%s/consent/%s", conf.AddrGatewayUrl, invite.Token))
	require.Nil(t, err)

	t.Run("as member", func(t *testing.T) {
		err := client.LeaveOrg(ctx2)
		require.Nil(t, err)
	})
}

func SkipTestClient_RegisterUser(t *testing.T) {
	t.Parallel()
	conf, client, done := setup(t)
	defer done()

	apitest.Login(t, client, conf, apitest.NewEmail())

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
	conf, shutdown := apitest.MakeTextile(t)
	defer shutdown()
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrApi)
	require.Nil(t, err)
	client, err := c.NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{}))
	require.Nil(t, err)

	err = client.Close()
	require.Nil(t, err)
}

func setup(t *testing.T) (core.Config, *c.Client, func()) {
	conf, shutdown := apitest.MakeTextile(t)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrApi)
	require.Nil(t, err)
	client, err := c.NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{}))
	require.Nil(t, err)

	return conf, client, func() {
		shutdown()
		err := client.Close()
		require.Nil(t, err)
	}
}
