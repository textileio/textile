package apitest

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"github.com/textileio/textile/v2/api/hubd/client"
	pb "github.com/textileio/textile/v2/api/hubd/pb"
	"github.com/textileio/textile/v2/core"
	"github.com/textileio/textile/v2/util"
)

const SessionSecret = "hubsession"

func MakeTextile(t *testing.T) core.Config {
	conf := DefaultTextileConfig(t)
	MakeTextileWithConfig(t, conf, true)
	return conf
}

func DefaultTextileConfig(t util.TestingTWithCleanup) core.Config {
	apiPort, err := freeport.GetFreePort()
	require.NoError(t, err)
	gatewayPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	return core.Config{
		Hub:                  true,
		Debug:                true,
		AddrAPI:              util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", apiPort)),
		AddrAPIProxy:         util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrMongoURI:         "mongodb://127.0.0.1:27017",
		AddrMongoName:        util.MakeToken(12),
		AddrThreadsHost:      util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrThreadsMongoURI:  "mongodb://127.0.0.1:27017",
		AddrThreadsMongoName: util.MakeToken(12),
		AddrIPFSAPI:          util.MustParseAddr("/ip4/127.0.0.1/tcp/5001"),
		AddrGatewayHost:      util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gatewayPort)),
		AddrGatewayURL:       fmt.Sprintf("http://127.0.0.1:%d", gatewayPort),
		EmailSessionSecret:   SessionSecret,
	}
}

func MakeTextileWithConfig(t util.TestingTWithCleanup, conf core.Config, autoShutdown bool) func() {
	textile, err := core.NewTextile(context.Background(), conf)
	require.NoError(t, err)
	textile.Bootstrap()
	time.Sleep(time.Second * time.Duration(rand.Float64()*5)) // Give the api a chance to get ready
	done := func() {
		time.Sleep(time.Second) // Give threads a chance to finish work
		err := textile.Close(true)
		require.NoError(t, err)
	}
	if autoShutdown {
		t.Cleanup(func() {
			done()
		})
	}
	return done
}

func NewUsername() string {
	return strings.ToLower(util.MakeToken(12))
}

func NewEmail() string {
	return fmt.Sprintf("%s@test.com", NewUsername())
}

func Signup(t util.TestingTWithCleanup, client *client.Client, conf core.Config, username, email string) *pb.SignupResponse {
	var err error
	var res *pb.SignupResponse
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

func Signin(t *testing.T, client *client.Client, conf core.Config, usernameOrEmail string) *pb.SigninResponse {
	var err error
	var res *pb.SigninResponse
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
