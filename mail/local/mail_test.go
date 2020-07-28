package local_test

import (
	"context"
	"crypto/rand"
	"io/ioutil"
	"os"
	"testing"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/api/apitest"
	"github.com/textileio/textile/api/common"
	hubpb "github.com/textileio/textile/api/hub/pb"
	"github.com/textileio/textile/cmd"
	. "github.com/textileio/textile/mail/local"
)

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

//func TestBuckets_NewConfigFromCmd(t *testing.T) {
//	buckets := setup(t)
//
//	t.Run("no flags", func(t *testing.T) {
//		c := initCmd(t, buckets, "", thread.Undef, false, false)
//		err := c.Execute()
//		require.NoError(t, err)
//	})
//
//	t.Run("with flags and no values", func(t *testing.T) {
//		c := initCmd(t, buckets, "", thread.Undef, true, false)
//		err := c.Execute()
//		require.NoError(t, err)
//	})
//
//	t.Run("with flags and default values", func(t *testing.T) {
//		key := "mykey"
//		tid := thread.NewIDV1(thread.Raw, 32)
//		c := initCmd(t, buckets, key, tid, true, true)
//		err := c.Execute()
//		require.NoError(t, err)
//	})
//
//	t.Run("with flags and set values", func(t *testing.T) {
//		key := "mykey"
//		tid := thread.NewIDV1(thread.Raw, 32)
//		c := initCmd(t, buckets, key, tid, true, false)
//		err := c.PersistentFlags().Set("key", key)
//		require.NoError(t, err)
//		err = c.PersistentFlags().Set("thread", tid.String())
//		require.NoError(t, err)
//		err = c.Execute()
//		require.NoError(t, err)
//	})
//
//	t.Run("no flags and env values", func(t *testing.T) {
//		key := "mykey"
//		tid := thread.NewIDV1(thread.Raw, 32)
//		c := initCmd(t, buckets, key, tid, false, false)
//		err := os.Setenv("BUCK_KEY", key)
//		require.NoError(t, err)
//		err = os.Setenv("BUCK_THREAD", tid.String())
//		require.NoError(t, err)
//		err = c.Execute()
//		require.NoError(t, err)
//	})
//
//	t.Run("with flags and env values", func(t *testing.T) {
//		key := "mykey"
//		tid := thread.NewIDV1(thread.Raw, 32)
//		c := initCmd(t, buckets, key, tid, true, false)
//		err := os.Setenv("BUCK_KEY", key)
//		require.NoError(t, err)
//		err = os.Setenv("BUCK_THREAD", tid.String())
//		require.NoError(t, err)
//		err = c.Execute()
//		require.NoError(t, err)
//	})
//
//	t.Run("with key and no thread", func(t *testing.T) {
//		dir := newDir(t)
//		c := &cobra.Command{
//			Use: "init",
//			Run: func(c *cobra.Command, args []string) {
//				_, err := buckets.NewConfigFromCmd(c, dir)
//				require.Error(t, err)
//				assert.Equal(t, ErrThreadRequired, err)
//			},
//		}
//		err := os.Setenv("BUCK_KEY", "mykey")
//		require.NoError(t, err)
//		err = os.Setenv("BUCK_THREAD", "")
//		require.NoError(t, err)
//		err = c.Execute()
//		require.NoError(t, err)
//	})
//}

//func initCmd(t *testing.T, buckets *Buckets, key string, tid thread.ID, addFlags, setDefaults bool) *cobra.Command {
//	dir := newDir(t)
//	c := &cobra.Command{
//		Use: "init",
//		Run: func(c *cobra.Command, args []string) {
//			conf, err := buckets.NewConfigFromCmd(c, dir)
//			require.NoError(t, err)
//			assert.Equal(t, dir, conf.Path)
//			assert.Equal(t, key, conf.Key)
//			if tid.Defined() {
//				assert.Equal(t, tid, conf.Thread)
//			} else {
//				assert.Equal(t, thread.Undef, conf.Thread)
//			}
//		},
//	}
//	var dkey, dtid string
//	if setDefaults {
//		dkey = key
//		dtid = tid.String()
//	}
//	if addFlags {
//		c.PersistentFlags().String("key", dkey, "")
//		//c.PersistentFlags().String("thread", dtid, "")
//	}
//	return c
//}

func setup(t *testing.T) (m *Mail, key string, secret string) {
	conf := apitest.MakeTextile(t)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.NoError(t, err)
	clients := cmd.NewClients(target, true)
	t.Cleanup(func() {
		clients.Close()
	})

	dev := apitest.Signup(t, clients.Hub, conf, apitest.NewUsername(), apitest.NewEmail())
	res, err := clients.Hub.CreateKey(common.NewSessionContext(context.Background(), dev.Session), hubpb.KeyType_USER, true)
	require.NoError(t, err)
	return NewMail(clients, DefaultConfConfig()), res.Key, res.Secret
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
