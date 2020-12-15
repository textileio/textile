package client_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/apitest"
	"github.com/textileio/textile/v2/api/common"
	c "github.com/textileio/textile/v2/api/hubd/client"
	pb "github.com/textileio/textile/v2/api/hubd/pb"
	"github.com/textileio/textile/v2/core"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	cleanup := func() {}
	if os.Getenv("SKIP_SERVICES") != "true" {
		cleanup = apitest.StartServices()
	}
	exitVal := m.Run()
	cleanup()
	os.Exit(exitVal)
}

func TestClient_Signup(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	assert.NotEmpty(t, user.Key)
	assert.NotEmpty(t, user.Session)
}

func TestClient_Signin(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)

	username := apitest.NewUsername()
	email := apitest.NewEmail()
	user := apitest.Signup(t, client, conf, username, email)
	err := client.Signout(common.NewSessionContext(context.Background(), user.Session))
	require.NoError(t, err)

	res := apitest.Signin(t, client, conf, username)
	assert.NotEmpty(t, res.Key)
	assert.NotEmpty(t, res.Session)

	err = client.Signout(common.NewSessionContext(context.Background(), res.Session))
	require.NoError(t, err)

	res = apitest.Signin(t, client, conf, email)
	assert.NotEmpty(t, res.Key)
	assert.NotEmpty(t, res.Session)
}

func TestClient_Signout(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)
	ctx := context.Background()

	t.Run("without session", func(t *testing.T) {
		err := client.Signout(ctx)
		require.Error(t, err)
	})

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("with session", func(t *testing.T) {
		err := client.Signout(common.NewSessionContext(ctx, user.Session))
		require.NoError(t, err)
	})
}

func TestClient_GetSessionInfo(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)
	ctx := context.Background()

	t.Run("without session", func(t *testing.T) {
		_, err := client.GetSessionInfo(ctx)
		require.Error(t, err)
	})

	username := apitest.NewUsername()
	email := apitest.NewEmail()
	user := apitest.Signup(t, client, conf, username, email)

	t.Run("with session", func(t *testing.T) {
		res, err := client.GetSessionInfo(common.NewSessionContext(ctx, user.Session))
		require.NoError(t, err)
		assert.Equal(t, user.Key, res.Key)
		assert.Equal(t, username, res.Username)
		assert.Equal(t, email, res.Email)
	})
}

func TestClient_GetIdentity(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)
	ctx := context.Background()

	t.Run("without session", func(t *testing.T) {
		_, err := client.GetIdentity(ctx)
		require.Error(t, err)
	})

	username := apitest.NewUsername()
	email := apitest.NewEmail()
	user := apitest.Signup(t, client, conf, username, email)

	t.Run("with session", func(t *testing.T) {
		res, err := client.GetIdentity(common.NewSessionContext(ctx, user.Session))
		require.NoError(t, err)
		id := &thread.Libp2pIdentity{}
		err = id.UnmarshalBinary(res.Identity)
		require.NoError(t, err)
		key := &thread.Libp2pPubKey{}
		err = key.UnmarshalBinary(user.Key)
		require.NoError(t, err)
		assert.True(t, id.GetPublic().Equals(key))
	})
}

func TestClient_CreateKey(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)
	ctx := context.Background()

	t.Run("without session", func(t *testing.T) {
		_, err := client.CreateKey(ctx, pb.KeyType_KEY_TYPE_ACCOUNT, true)
		require.Error(t, err)
	})

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("with session", func(t *testing.T) {
		res, err := client.CreateKey(common.NewSessionContext(ctx, user.Session), pb.KeyType_KEY_TYPE_ACCOUNT, true)
		require.NoError(t, err)
		assert.NotEmpty(t, res.KeyInfo.Key)
		assert.NotEmpty(t, res.KeyInfo.Secret)
		assert.Equal(t, pb.KeyType_KEY_TYPE_ACCOUNT, res.KeyInfo.Type)
		assert.True(t, res.KeyInfo.Secure)
	})
}

func TestClient_InvalidateKey(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)
	ctx := context.Background()

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	res, err := client.CreateKey(common.NewSessionContext(ctx, user.Session), pb.KeyType_KEY_TYPE_ACCOUNT, true)
	require.NoError(t, err)

	t.Run("without session", func(t *testing.T) {
		err := client.InvalidateKey(ctx, res.KeyInfo.Key)
		require.Error(t, err)
	})

	ctx = common.NewSessionContext(ctx, user.Session)

	t.Run("with session", func(t *testing.T) {
		err := client.InvalidateKey(ctx, res.KeyInfo.Key)
		require.NoError(t, err)
		keys, err := client.ListKeys(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, len(keys.List))
		require.False(t, keys.List[0].Valid)
	})
}

func TestClient_ListKeys(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)

	t.Run("empty", func(t *testing.T) {
		keys, err := client.ListKeys(ctx)
		require.NoError(t, err)
		assert.Empty(t, keys.List)
	})

	_, err := client.CreateKey(ctx, pb.KeyType_KEY_TYPE_ACCOUNT, true)
	require.NoError(t, err)
	_, err = client.CreateKey(ctx, pb.KeyType_KEY_TYPE_USER, true)
	require.NoError(t, err)

	t.Run("not empty", func(t *testing.T) {
		keys, err := client.ListKeys(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, len(keys.List))
	})
}

func TestClient_CreateOrg(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)
	ctx := context.Background()

	name := apitest.NewUsername()

	t.Run("without session", func(t *testing.T) {
		_, err := client.CreateOrg(ctx, name)
		require.Error(t, err)
	})

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("with session", func(t *testing.T) {
		res, err := client.CreateOrg(common.NewSessionContext(ctx, user.Session), name)
		require.NoError(t, err)
		assert.NotEmpty(t, res.OrgInfo.Key)
		assert.Equal(t, name, res.OrgInfo.Name)
	})
}

func TestClient_GetOrg(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)

	name := apitest.NewUsername()
	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	res, err := client.CreateOrg(ctx, name)
	require.NoError(t, err)

	t.Run("bad org", func(t *testing.T) {
		_, err := client.GetOrg(common.NewOrgSlugContext(ctx, "bad"))
		require.Error(t, err)
	})

	t.Run("good org", func(t *testing.T) {
		got, err := client.GetOrg(common.NewOrgSlugContext(ctx, res.OrgInfo.Name))
		require.NoError(t, err)
		assert.Equal(t, res.OrgInfo.Key, got.OrgInfo.Key)
	})
}

func TestClient_ListOrgs(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)

	t.Run("empty", func(t *testing.T) {
		orgs, err := client.ListOrgs(ctx)
		require.NoError(t, err)
		assert.Empty(t, orgs.List)
	})

	_, err := client.CreateOrg(ctx, "My Org 1")
	require.NoError(t, err)
	_, err = client.CreateOrg(ctx, "My Org 2")
	require.NoError(t, err)

	t.Run("not empty", func(t *testing.T) {
		orgs, err := client.ListOrgs(ctx)
		require.NoError(t, err)
		assert.Equal(t, 2, len(orgs.List))
	})
}

func TestClient_RemoveOrg(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)

	name := apitest.NewUsername()
	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	res, err := client.CreateOrg(ctx, name)
	require.NoError(t, err)

	t.Run("bad org", func(t *testing.T) {
		err := client.RemoveOrg(common.NewOrgSlugContext(ctx, "bad"))
		require.Error(t, err)
	})

	user2 := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx2 := common.NewSessionContext(context.Background(), user2.Session)

	t.Run("bad session", func(t *testing.T) {
		err := client.RemoveOrg(common.NewOrgSlugContext(ctx2, res.OrgInfo.Name))
		require.Error(t, err)
	})

	t.Run("good org", func(t *testing.T) {
		octx := common.NewOrgSlugContext(ctx, res.OrgInfo.Name)
		err := client.RemoveOrg(octx)
		require.NoError(t, err)
		_, err = client.GetOrg(octx)
		require.Error(t, err)
	})
}

func TestClient_InviteToOrg(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)

	name := apitest.NewUsername()
	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	res, err := client.CreateOrg(ctx, name)
	require.NoError(t, err)
	ctx = common.NewOrgSlugContext(ctx, res.OrgInfo.Name)

	t.Run("bad email", func(t *testing.T) {
		_, err := client.InviteToOrg(ctx, "jane")
		require.Error(t, err)
	})

	t.Run("good email", func(t *testing.T) {
		res, err := client.InviteToOrg(ctx, apitest.NewEmail())
		require.NoError(t, err)
		assert.NotEmpty(t, res.Token)
	})
}

func TestClient_LeaveOrg(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)

	name := apitest.NewUsername()
	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	res, err := client.CreateOrg(ctx, name)
	require.NoError(t, err)
	ctx = common.NewOrgSlugContext(ctx, res.OrgInfo.Name)

	t.Run("as owner", func(t *testing.T) {
		err := client.LeaveOrg(ctx)
		require.Error(t, err)
	})

	user2Email := apitest.NewEmail()
	user2 := apitest.Signup(t, client, conf, apitest.NewUsername(), user2Email)
	ctx2 := common.NewSessionContext(ctx, user2.Session)

	t.Run("as non-member", func(t *testing.T) {
		err := client.LeaveOrg(ctx2)
		require.Error(t, err)
	})

	invite, err := client.InviteToOrg(ctx, user2Email)
	require.NoError(t, err)
	_, err = http.Get(fmt.Sprintf("%s/consent/%s", conf.AddrGatewayURL, invite.Token))
	require.NoError(t, err)

	t.Run("as member", func(t *testing.T) {
		err := client.LeaveOrg(ctx2)
		require.NoError(t, err)
	})
}

func TestClient_SetupBilling(t *testing.T) {
	t.Parallel()
	conf, client, _ := setupWithBilling(t)
	ctx := context.Background()

	t.Run("without session", func(t *testing.T) {
		err := client.SetupBilling(ctx)
		require.Error(t, err)
	})

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("with session", func(t *testing.T) {
		err := client.SetupBilling(common.NewSessionContext(ctx, user.Session))
		require.Error(t, err)
		assert.True(t, strings.Contains(err.Error(), "subscription exists"))
		// Note: There's no way API for canceling the subscription since users
		// will do that via the Stripe customer portal. So, for now we're not able
		// to have a test for re-enabling billing after they cancel.
	})
}

func TestClient_GetBillingSession(t *testing.T) {
	t.Parallel()
	conf, client, _ := setupWithBilling(t)
	ctx := context.Background()

	t.Run("without session", func(t *testing.T) {
		_, err := client.GetBillingSession(ctx)
		require.Error(t, err)
	})

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())

	t.Run("with session", func(t *testing.T) {
		res, err := client.GetBillingSession(common.NewSessionContext(ctx, user.Session))
		require.NoError(t, err)
		assert.NotEmpty(t, res.Url)
	})
}

func TestClient_IsUsernameAvailable(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)

	username := apitest.NewUsername()
	err := client.IsUsernameAvailable(context.Background(), username)
	require.NoError(t, err)

	apitest.Signup(t, client, conf, username, apitest.NewEmail())

	err = client.IsUsernameAvailable(context.Background(), username)
	require.Error(t, err)
}

func TestClient_IsOrgNameAvailable(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)

	user := apitest.Signup(t, client, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)

	name := "My awesome org!"
	res, err := client.IsOrgNameAvailable(ctx, name)
	require.NoError(t, err)
	require.Equal(t, "My-awesome-org", res.Slug)

	res2, err := client.CreateOrg(ctx, name)
	require.NoError(t, err)
	require.Equal(t, res.Slug, res2.OrgInfo.Slug)

	_, err = client.IsOrgNameAvailable(ctx, name)
	require.Error(t, err)
}

func TestClient_DestroyAccount(t *testing.T) {
	t.Parallel()
	conf, client, _ := setup(t, nil)

	username := apitest.NewUsername()
	user := apitest.Signup(t, client, conf, username, apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)

	err := client.DestroyAccount(ctx)
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, err = client.Signin(context.Background(), username)
	}()
	apitest.ConfirmEmail(t, conf.AddrGatewayURL, apitest.SessionSecret)
	wg.Wait()
	require.Error(t, err)
}

func TestClose(t *testing.T) {
	t.Parallel()
	conf := apitest.MakeTextile(t)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.NoError(t, err)
	client, err := c.NewClient(target, grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{}))
	require.NoError(t, err)

	err = client.Close()
	require.NoError(t, err)
}

func setup(t *testing.T, conf *core.Config) (core.Config, *c.Client, *tc.Client) {
	if conf == nil {
		tmp := apitest.DefaultTextileConfig(t)
		conf = &tmp
	}
	apitest.MakeTextileWithConfig(t, *conf)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.NoError(t, err)
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{})}
	client, err := c.NewClient(target, opts...)
	require.NoError(t, err)
	threadsclient, err := tc.NewClient(target, opts...)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := client.Close()
		require.NoError(t, err)
		err = threadsclient.Close()
		require.NoError(t, err)
	})
	return *conf, client, threadsclient
}

func setupWithBilling(t *testing.T) (core.Config, *c.Client, *tc.Client) {
	bconf := apitest.DefaultBillingConfig(t)
	apitest.MakeBillingWithConfig(t, bconf)

	conf := apitest.DefaultTextileConfig(t)
	billingApi, err := tutil.TCPAddrFromMultiAddr(bconf.ListenAddr)
	require.NoError(t, err)
	conf.AddrBillingAPI = billingApi
	return setup(t, &conf)
}
