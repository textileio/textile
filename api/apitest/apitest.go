package apitest

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"github.com/textileio/textile/api/hub/client"
	pb "github.com/textileio/textile/api/hub/pb"
	"github.com/textileio/textile/core"
	"github.com/textileio/textile/util"
)

var SessionSecret = NewUsername()

func MakeTextile(t *testing.T) (conf core.Config, shutdown func()) {
	time.Sleep(time.Second * time.Duration(rand.Intn(5)))

	dir, err := ioutil.TempDir("", "")
	require.Nil(t, err)

	apiPort, err := freeport.GetFreePort()
	require.Nil(t, err)
	gatewayPort, err := freeport.GetFreePort()
	require.Nil(t, err)

	conf = core.Config{
		RepoPath: dir,

		AddrAPI:         util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", apiPort)),
		AddrAPIProxy:    util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrThreadsHost: util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrIPFSAPI:     util.MustParseAddr("/ip4/127.0.0.1/tcp/5001"),
		AddrGatewayHost: util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gatewayPort)),
		AddrGatewayURL:  fmt.Sprintf("http://127.0.0.1:%d", gatewayPort),
		AddrMongoURI:    "mongodb://127.0.0.1:27017",

		MongoName: util.MakeToken(12),

		EmailFrom:   "test@email.textile.io",
		EmailDomain: "email.textile.io",
		EmailAPIKey: "",

		SessionSecret: SessionSecret,

		Debug: true,
	}
	textile, err := core.NewTextile(context.Background(), conf)
	require.Nil(t, err)
	textile.Bootstrap()

	return conf, func() {
		time.Sleep(time.Second) // Give threads a chance to finish work
		err := textile.Close()
		require.Nil(t, err)
		_ = os.RemoveAll(dir)
	}
}

func NewUsername() string {
	return strings.ToLower(util.MakeToken(12))
}

func NewEmail() string {
	return fmt.Sprintf("%s@doe.com", NewUsername())
}

func Signup(t *testing.T, client *client.Client, conf core.Config, username, email string) *pb.SignupReply {
	var err error
	var res *pb.SignupReply
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		res, err = client.Signup(context.Background(), username, email)
		require.Nil(t, err)
	}()
	ConfirmEmail(t, conf.AddrGatewayURL, SessionSecret)
	wg.Wait()
	require.NotNil(t, res)
	require.NotEmpty(t, res.Session)
	return res
}

func Signin(t *testing.T, client *client.Client, conf core.Config, usernameOrEmail string) *pb.SigninReply {
	var err error
	var res *pb.SigninReply
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		res, err = client.Signin(context.Background(), usernameOrEmail)
		require.Nil(t, err)
	}()
	ConfirmEmail(t, conf.AddrGatewayURL, SessionSecret)
	wg.Wait()
	require.NotNil(t, res)
	require.NotEmpty(t, res.Session)
	return res
}

func ConfirmEmail(t *testing.T, gurl string, secret string) {
	time.Sleep(time.Second)
	url := fmt.Sprintf("%s/confirm/%s", gurl, secret)
	_, err := http.Get(url)
	require.Nil(t, err)
	time.Sleep(time.Second)
}
