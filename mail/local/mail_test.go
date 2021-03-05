package local_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"io/ioutil"
	"os"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/apitest"
	"github.com/textileio/textile/v2/api/common"
	hubpb "github.com/textileio/textile/v2/api/hubd/pb"
	"github.com/textileio/textile/v2/cmd"
	. "github.com/textileio/textile/v2/mail/local"
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

func TestMail_NewMailbox(t *testing.T) {
	mail, key, secret := setup(t)

	t.Run("new mailbox", func(t *testing.T) {
		conf := getConf(t, key, secret)
		box, err := mail.NewMailbox(context.Background(), conf)
		require.NoError(t, err)
		assert.NotEmpty(t, box)

		reloaded, err := mail.GetLocalMailbox(context.Background(), conf.Path)
		require.NoError(t, err)
		assert.NotEmpty(t, reloaded)
	})
}

func TestBuckets_NewConfigFromCmd(t *testing.T) {
	mail, key, secret := setup(t)
	id := createIdentity(t)

	t.Run("no identity", func(t *testing.T) {
		dir := newDir(t)
		c := &cobra.Command{
			Use: "init",
			Run: func(c *cobra.Command, args []string) {
				_, err := mail.NewConfigFromCmd(c, dir)
				require.Error(t, err)
				assert.Equal(t, ErrIdentityRequired, err)
			},
		}
		err := c.Execute()
		require.NoError(t, err)
	})

	t.Run("no api key", func(t *testing.T) {
		dir := newDir(t)
		c := &cobra.Command{
			Use: "init",
			Run: func(c *cobra.Command, args []string) {
				_, err := mail.NewConfigFromCmd(c, dir)
				require.Error(t, err)
				assert.Equal(t, ErrAPIKeyRequired, err)
			},
		}
		c.PersistentFlags().String("identity", "", "")
		idb, err := id.MarshalBinary()
		require.NoError(t, err)
		ids := base64.StdEncoding.EncodeToString(idb)
		err = c.PersistentFlags().Set("identity", ids)
		require.NoError(t, err)
		err = c.Execute()
		require.NoError(t, err)
	})

	t.Run("with flags and set values", func(t *testing.T) {
		c := initCmd(t, mail, id, key, secret, true, false)
		idb, err := id.MarshalBinary()
		require.NoError(t, err)
		ids := base64.StdEncoding.EncodeToString(idb)
		err = c.PersistentFlags().Set("identity", ids)
		require.NoError(t, err)
		err = c.PersistentFlags().Set("api_key", key)
		require.NoError(t, err)
		err = c.PersistentFlags().Set("api_secret", secret)
		require.NoError(t, err)
		err = c.Execute()
		require.NoError(t, err)
	})

	t.Run("no flags and env values", func(t *testing.T) {
		c := initCmd(t, mail, id, key, secret, false, false)
		idb, err := id.MarshalBinary()
		require.NoError(t, err)
		ids := base64.StdEncoding.EncodeToString(idb)
		err = os.Setenv("MAIL_IDENTITY", ids)
		require.NoError(t, err)
		err = os.Setenv("MAIL_API_KEY", key)
		require.NoError(t, err)
		err = os.Setenv("MAIL_API_SECRET", secret)
		require.NoError(t, err)
		err = c.Execute()
		require.NoError(t, err)
	})

	t.Run("with flags and env values", func(t *testing.T) {
		c := initCmd(t, mail, id, key, secret, true, false)
		idb, err := id.MarshalBinary()
		require.NoError(t, err)
		ids := base64.StdEncoding.EncodeToString(idb)
		err = os.Setenv("MAIL_IDENTITY", ids)
		require.NoError(t, err)
		err = os.Setenv("MAIL_API_KEY", key)
		require.NoError(t, err)
		err = os.Setenv("MAIL_API_SECRET", secret)
		require.NoError(t, err)
		err = c.Execute()
		require.NoError(t, err)
	})
}

func initCmd(
	t *testing.T,
	mail *Mail,
	id thread.Identity,
	key, secret string,
	addFlags,
	setDefaults bool,
) *cobra.Command {
	dir := newDir(t)
	c := &cobra.Command{
		Use: "init",
		Run: func(c *cobra.Command, args []string) {
			conf, err := mail.NewConfigFromCmd(c, dir)
			require.NoError(t, err)
			assert.Equal(t, dir, conf.Path)
			assert.Equal(t, id.GetPublic().String(), conf.Identity.GetPublic().String())
			assert.Equal(t, key, conf.APIKey)
			assert.Equal(t, secret, conf.APISecret)
		},
	}
	var did, dkey, dsecret string
	if setDefaults {
		idb, err := id.MarshalBinary()
		require.NoError(t, err)
		did = base64.StdEncoding.EncodeToString(idb)
		dkey = key
		dsecret = secret
	}
	if addFlags {
		c.PersistentFlags().String("identity", did, "")
		c.PersistentFlags().String("api_key", dkey, "")
		c.PersistentFlags().String("api_secret", dsecret, "")
	}
	return c
}

func setup(t *testing.T) (m *Mail, key string, secret string) {
	conf := apitest.MakeTextile(t)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.NoError(t, err)
	clients := cmd.NewClients(target, true, "")
	t.Cleanup(func() {
		clients.Close()
	})

	dev := apitest.Signup(t, clients.Hub, conf, apitest.NewUsername(), apitest.NewEmail())
	res, err := clients.Hub.CreateKey(
		common.NewSessionContext(context.Background(), dev.Session),
		hubpb.KeyType_KEY_TYPE_USER,
		true,
	)
	require.NoError(t, err)
	return NewMail(clients, DefaultConfConfig()), res.KeyInfo.Key, res.KeyInfo.Secret
}

func getConf(t *testing.T, key, secret string) Config {
	return Config{
		Path:      newDir(t),
		Identity:  createIdentity(t),
		APIKey:    key,
		APISecret: secret,
	}
}

func createIdentity(t *testing.T) thread.Identity {
	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return thread.NewLibp2pIdentity(sk)
}

func newDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}
