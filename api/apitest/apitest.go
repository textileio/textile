package apitest

import (
	"context"
	"crypto/tls"
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
	stripe "github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/token"
	"github.com/textileio/textile/v2/api/hub/client"
	pb "github.com/textileio/textile/v2/api/hub/pb"
	"github.com/textileio/textile/v2/core"
	"github.com/textileio/textile/v2/util"
	"golang.org/x/net/http2"
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
		AddrMongoURI:    "mongodb://127.0.0.1:27017/?replicaSet=rs0",

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

func ConfigureStripe(t *testing.T, url, key string) {
	stripe.Key = key
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	err := http2.ConfigureTransport(transport)
	require.NoError(t, err)
	stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(
		stripe.APIBackend,
		&stripe.BackendConfig{
			URL: stripe.String(url),
			HTTPClient: &http.Client{
				Transport: transport,
			},
			LeveledLogger: stripe.DefaultLeveledLogger,
		},
	))
}

func NewCardToken(t *testing.T) string {
	ConfigureStripe(t, "https://api.stripe.com", "sk_test_RuU6Lq65WP23ykDSI9N9nRbC")
	tok, err := token.New(&stripe.TokenParams{
		Card: &stripe.CardParams{
			Number:   stripe.String("4242424242424242"),
			ExpMonth: stripe.String("12"),
			ExpYear:  stripe.String("2021"),
			CVC:      stripe.String("123"),
		},
	})
	require.NoError(t, err)
	return tok.ID
}
