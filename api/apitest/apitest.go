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

const SessionSecret = "hubsession"

func MakeTextile(t *testing.T) core.Config {
	conf := DefaultTextileConfig(t)
	MakeTextileWithConfig(t, conf, true)
	return conf
}

func DefaultTextileConfig(t util.TestingTWithCleanup) core.Config {
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	apiPort, err := freeport.GetFreePort()
	require.NoError(t, err)
	gatewayPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	return core.Config{
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

		EmailSessionSecret: SessionSecret,

		Hub:   true,
		Debug: true,
	}
}

func MakeTextileWithConfig(t util.TestingTWithCleanup, conf core.Config, autoShutdown bool) func(deleteRepo bool) {
	textile, err := core.NewTextile(context.Background(), conf)
	require.NoError(t, err)
	textile.Bootstrap()
	time.Sleep(time.Second * time.Duration(rand.Float64()*5)) // Give the api a chance to get ready
	done := func(deleteRepo bool) {
		time.Sleep(time.Second) // Give threads a chance to finish work
		err := textile.Close(true)
		require.NoError(t, err)
		if deleteRepo {
			_ = os.RemoveAll(conf.RepoPath)
		}
	}
	if autoShutdown {
		t.Cleanup(func() {
			done(true)
		})
	}
	return done
}

func NewUsername() string {
	return strings.ToLower(util.MakeToken(12))
}

func NewEmail() string {
	return fmt.Sprintf("%s@doe.com", NewUsername())
}

func Signup(t util.TestingTWithCleanup, client *client.Client, conf core.Config, username, email string) *pb.SignupReply {
	var err error
	var res *pb.SignupReply
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		res, err = client.Signup(context.Background(), username, email)
		require.NoError(t, err)
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
		require.NoError(t, err)
	}()
	ConfirmEmail(t, conf.AddrGatewayURL, SessionSecret)
	wg.Wait()
	require.NotNil(t, res)
	require.NotEmpty(t, res.Session)
	return res
}

func ConfirmEmail(t util.TestingTWithCleanup, gurl string, secret string) {
	time.Sleep(time.Second)
	url := fmt.Sprintf("%s/confirm/%s", gurl, secret)
	_, err := http.Get(url)
	require.NoError(t, err)
	time.Sleep(time.Second)
}
