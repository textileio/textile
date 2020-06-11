package client_test

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/textileio/go-threads/api/client"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/apitest"
	"github.com/textileio/textile/api/common"
	c "github.com/textileio/textile/api/hub/client"
	pb "github.com/textileio/textile/api/hub/pb"
	"github.com/textileio/textile/core"
	"google.golang.org/grpc"
)

func TestClient_Signup(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	assert.NotEmpty(t, user.Key)
	assert.NotEmpty(t, user.Session)
}

func TestClient_Signin(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()

	username := apitest.NewUsername()
	email := apitest.NewEmail()
	user := apitest.Signup(t, client, conf, username, email)
	err := client.Signout(common.NewSessionContext(context.Background(), user.Session))
	require.Nil(t, err)

	res := apitest.Signin(t, client, conf, username)
	assert.NotEmpty(t, res.Key)
	assert.NotEmpty(t, res.Session)

	err = client.Signout(common.NewSessionContext(context.Background(), res.Session))
	require.Nil(t, err)

	res = apitest.Signin(t, client, conf, email)
	assert.NotEmpty(t, res.Key)
	assert.NotEmpty(t, res.Session)
}

func TestClient_Signout(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()
	ctx := context.Background()

	t.Run("without session", func(t *testing.T) {
		err := client.Signout(ctx)
		require.NotNil(t, err)
	})

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("with session", func(t *testing.T) {
		err := client.Signout(common.NewSessionContext(ctx, user.Session))
		require.Nil(t, err)
	})
}

func TestClient_GetSessionInfo(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()
	ctx := context.Background()

	t.Run("without session", func(t *testing.T) {
		_, err := client.GetSessionInfo(ctx)
		require.NotNil(t, err)
	})

	username := apitest.NewUsername()
	email := apitest.NewEmail()
	user := apitest.Signup(t, client, conf, username, email)

	t.Run("with session", func(t *testing.T) {
		res, err := client.GetSessionInfo(common.NewSessionContext(ctx, user.Session))
		require.Nil(t, err)
		assert.Equal(t, user.Key, res.Key)
		assert.Equal(t, username, res.Username)
		assert.Equal(t, email, res.Email)
	})
}

func TestClient_CreateKey(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()
	ctx := context.Background()

	t.Run("without session", func(t *testing.T) {
		_, err := client.CreateKey(ctx, pb.KeyType_ACCOUNT, true)
		require.NotNil(t, err)
	})

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("with session", func(t *testing.T) {
		key, err := client.CreateKey(common.NewSessionContext(ctx, user.Session), pb.KeyType_ACCOUNT, true)
		require.Nil(t, err)
		assert.NotEmpty(t, key.Key)
		assert.NotEmpty(t, key.Secret)
		assert.Equal(t, pb.KeyType_ACCOUNT, key.Type)
		assert.True(t, key.Secure)
	})
}

func TestClient_InvalidateKey(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()
	ctx := context.Background()

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	key, err := client.CreateKey(common.NewSessionContext(ctx, user.Session), pb.KeyType_ACCOUNT, true)
	require.Nil(t, err)

	t.Run("without session", func(t *testing.T) {
		err := client.InvalidateKey(ctx, key.Key)
		require.NotNil(t, err)
	})

	ctx = common.NewSessionContext(ctx, user.Session)

	t.Run("with session", func(t *testing.T) {
		err := client.InvalidateKey(ctx, key.Key)
		require.Nil(t, err)
		keys, err := client.ListKeys(ctx)
		require.Nil(t, err)
		require.Equal(t, 1, len(keys.List))
		require.False(t, keys.List[0].Valid)
	})
}

func TestClient_ListKeys(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)

	t.Run("empty", func(t *testing.T) {
		keys, err := client.ListKeys(ctx)
		require.Nil(t, err)
		assert.Empty(t, keys.List)
	})

	_, err := client.CreateKey(ctx, pb.KeyType_ACCOUNT, true)
	require.Nil(t, err)
	_, err = client.CreateKey(ctx, pb.KeyType_USER, true)
	require.Nil(t, err)

	t.Run("not empty", func(t *testing.T) {
		keys, err := client.ListKeys(ctx)
		require.Nil(t, err)
		assert.Equal(t, 2, len(keys.List))
	})
}

func TestClient_CreateOrg(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()
	ctx := context.Background()

	name := apitest.NewUsername()

	t.Run("without session", func(t *testing.T) {
		_, err := client.CreateOrg(ctx, name)
		require.NotNil(t, err)
	})

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("with session", func(t *testing.T) {
		org, err := client.CreateOrg(common.NewSessionContext(ctx, user.Session), name)
		require.Nil(t, err)
		assert.NotEmpty(t, org.Key)
		assert.Equal(t, name, org.Name)
	})
}

func TestClient_GetOrg(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()

	name := apitest.NewUsername()
	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	org, err := client.CreateOrg(ctx, name)
	require.Nil(t, err)

	t.Run("bad org", func(t *testing.T) {
		_, err := client.GetOrg(common.NewOrgSlugContext(ctx, "bad"))
		require.NotNil(t, err)
	})

	t.Run("good org", func(t *testing.T) {
		got, err := client.GetOrg(common.NewOrgSlugContext(ctx, org.Name))
		require.Nil(t, err)
		assert.Equal(t, org.Key, got.Key)
	})
}

func TestClient_ListOrgs(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)

	t.Run("empty", func(t *testing.T) {
		orgs, err := client.ListOrgs(ctx)
		require.Nil(t, err)
		assert.Empty(t, orgs.List)
	})

	_, err := client.CreateOrg(ctx, "My Org 1")
	require.Nil(t, err)
	_, err = client.CreateOrg(ctx, "My Org 2")
	require.Nil(t, err)

	t.Run("not empty", func(t *testing.T) {
		orgs, err := client.ListOrgs(ctx)
		require.Nil(t, err)
		assert.Equal(t, 2, len(orgs.List))
	})
}

func TestClient_RemoveOrg(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()

	name := apitest.NewUsername()
	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	org, err := client.CreateOrg(ctx, name)
	require.Nil(t, err)

	t.Run("bad org", func(t *testing.T) {
		err := client.RemoveOrg(common.NewOrgSlugContext(ctx, "bad"))
		require.NotNil(t, err)
	})

	user2 := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx2 := common.NewSessionContext(context.Background(), user2.Session)

	t.Run("bad session", func(t *testing.T) {
		err := client.RemoveOrg(common.NewOrgSlugContext(ctx2, org.Name))
		require.NotNil(t, err)
	})

	t.Run("good org", func(t *testing.T) {
		octx := common.NewOrgSlugContext(ctx, org.Name)
		err := client.RemoveOrg(octx)
		require.Nil(t, err)
		_, err = client.GetOrg(octx)
		require.NotNil(t, err)
	})
}

func TestClient_InviteToOrg(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()

	name := apitest.NewUsername()
	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	org, err := client.CreateOrg(ctx, name)
	require.Nil(t, err)
	ctx = common.NewOrgSlugContext(ctx, org.Name)

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
	conf, client, _, done := setup(t)
	defer done()

	name := apitest.NewUsername()
	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	org, err := client.CreateOrg(ctx, name)
	require.Nil(t, err)
	ctx = common.NewOrgSlugContext(ctx, org.Name)

	t.Run("as owner", func(t *testing.T) {
		err := client.LeaveOrg(ctx)
		require.NotNil(t, err)
	})

	user2Email := apitest.NewEmail()
	user2 := apitest.Signup(t, client, conf, apitest.NewUsername(), user2Email)
	ctx2 := common.NewSessionContext(ctx, user2.Session)

	t.Run("as non-member", func(t *testing.T) {
		err := client.LeaveOrg(ctx2)
		require.NotNil(t, err)
	})

	invite, err := client.InviteToOrg(ctx, user2Email)
	require.Nil(t, err)
	_, err = http.Get(fmt.Sprintf("%s/consent/%s", conf.AddrGatewayURL, invite.Token))
	require.Nil(t, err)

	t.Run("as member", func(t *testing.T) {
		err := client.LeaveOrg(ctx2)
		require.Nil(t, err)
	})
}

func TestClient_IsUsernameAvailable(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()

	username := apitest.NewUsername()
	err := client.IsUsernameAvailable(context.Background(), username)
	require.Nil(t, err)

	apitest.Signup(t, client, conf, username, apitest.NewEmail())

	err = client.IsUsernameAvailable(context.Background(), username)
	require.NotNil(t, err)
}

func TestClient_IsOrgNameAvailable(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)

	name := "My awesome org!"
	res, err := client.IsOrgNameAvailable(ctx, name)
	require.Nil(t, err)
	require.Equal(t, "My-awesome-org", res.Slug)

	org, err := client.CreateOrg(ctx, name)
	require.Nil(t, err)
	require.Equal(t, res.Slug, org.Slug)

	_, err = client.IsOrgNameAvailable(ctx, name)
	require.NotNil(t, err)
}

func TestClient_DestroyAccount(t *testing.T) {
	t.Parallel()
	conf, client, _, done := setup(t)
	defer done()

	username := apitest.NewUsername()
	user := apitest.Signup(t, client, conf, username, apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)

	err := client.DestroyAccount(ctx)
	require.Nil(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err = client.Signin(context.Background(), username)
	}()
	apitest.ConfirmEmail(t, conf.AddrGatewayURL, apitest.SessionSecret)
	wg.Wait()
	require.NotNil(t, err)
}

func TestClose(t *testing.T) {
	t.Parallel()
	conf, shutdown := apitest.MakeTextile(t)
	defer shutdown()
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.Nil(t, err)
	client, err := c.NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{}))
	require.Nil(t, err)

	err = client.Close()
	require.Nil(t, err)
}

func setup(t *testing.T) (core.Config, *c.Client, *tc.Client, func()) {
	conf, shutdown := apitest.MakeTextile(t)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.Nil(t, err)
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{})}
	client, err := c.NewClient(target, opts...)
	require.Nil(t, err)
	threadsclient, err := tc.NewClient(target, opts...)
	require.Nil(t, err)

	return conf, client, threadsclient, func() {
		shutdown()
		err := client.Close()
		require.Nil(t, err)
		err = threadsclient.Close()
		require.Nil(t, err)
	}
}
