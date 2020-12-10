package apitest

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	httpapi "github.com/ipfs/go-ipfs-http-client"

	"github.com/textileio/go-ds-mongo/test"
	billing "github.com/textileio/textile/v2/api/billingd/service"

	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"github.com/textileio/textile/v2/api/hubd/client"
	pb "github.com/textileio/textile/v2/api/hubd/pb"
	"github.com/textileio/textile/v2/core"
	"github.com/textileio/textile/v2/util"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
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
		AddrMongoURI:         MongoUri,
		AddrMongoName:        util.MakeToken(12),
		AddrThreadsHost:      util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrThreadsMongoURI:  MongoUri,
		AddrThreadsMongoName: util.MakeToken(12),
		AddrIPFSAPI:          IPFSApiAddr,
		AddrGatewayHost:      util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gatewayPort)),
		AddrGatewayURL:       fmt.Sprintf("http://127.0.0.1:%d", gatewayPort),
		EmailSessionSecret:   SessionSecret,
	}
}

func MakeTextileWithConfig(t util.TestingTWithCleanup, conf core.Config, autoShutdown bool) func() {
	textile, err := core.NewTextile(context.Background(), conf)
	require.NoError(t, err)
	time.Sleep(time.Second * time.Duration(rand.Float64()*5)) // Give the api a chance to get ready
	done := func() {
		time.Sleep(time.Second) // Give threads a chance to finish work
		err := textile.Close()
		require.NoError(t, err)
	}
	if autoShutdown {
		t.Cleanup(func() {
			done()
		})
	}
	return done
}

func DefaultBillingConfig(t util.TestingTWithCleanup) billing.Config {
	apiPort, err := freeport.GetFreePort()
	require.NoError(t, err)
	gatewayPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	return billing.Config{
		ListenAddr:             util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", apiPort)),
		StripeAPIURL:           "https://api.stripe.com",
		StripeAPIKey:           os.Getenv("STRIPE_API_KEY"),
		StripeSessionReturnURL: "http://127.0.0.1:8006/dashboard",
		SegmentAPIKey:          os.Getenv("SEGMENT_API_KEY"),
		SegmentPrefix:          "test_",
		DBURI:                  test.MongoUri,
		DBName:                 util.MakeToken(8),
		GatewayHostAddr:        util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gatewayPort)),
		Debug:                  true,
	}
}

func MakeBillingWithConfig(t util.TestingTWithCleanup, conf billing.Config) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	api, err := billing.NewService(ctx, conf)
	require.NoError(t, err)
	err = api.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		err := api.Stop(true)
		require.NoError(t, err)
	})
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

var (
	MongoUri     = "mongodb://127.0.0.1:27027"
	IPFSApiAddr  = util.MustParseAddr("/ip4/127.0.0.1/tcp/5011")
	IPFSHostAddr = util.MustParseAddr("/ip4/127.0.0.1/tcp/4011")
)

// StartServices starts local mongodb and ipfs services.
func StartServices() (cleanup func()) {
	_, currentFilePath, _, _ := runtime.Caller(0)
	dirpath := path.Dir(currentFilePath)

	makeDown := func() {
		cmd := exec.Command(
			"docker-compose",
			"-f",
			fmt.Sprintf("%s/docker-compose.yml", dirpath),
			"down", "-v",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatalf("docker-compose down: %s", err)
		}
	}
	makeDown()

	cmd := exec.Command(
		"docker-compose",
		"-f",
		fmt.Sprintf("%s/docker-compose.yml", dirpath),
		"build",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("docker-compose build: %s", err)
	}
	cmd = exec.Command(
		"docker-compose",
		"-f",
		fmt.Sprintf("%s/docker-compose.yml", dirpath),
		"up",
		"-V",
	)
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatalf("running docker-compose: %s", err)
	}

	limit := 10
	retries := 0
	var err error
	for retries < limit {
		err = checkServices()
		if err == nil {
			break
		}
		time.Sleep(time.Second)
		retries++
	}
	if retries == limit {
		if err != nil {
			log.Fatalf("connecting to services: %s", err)
		}
		log.Fatalf("max retries exhausted connecting to services")
	}
	return makeDown
}

func checkServices() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	_, err := mongo.Connect(ctx, options.Client().ApplyURI(MongoUri))
	if err != nil {
		return err
	}
	_, err = httpapi.NewApi(IPFSApiAddr)
	return err
}
