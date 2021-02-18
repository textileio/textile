package apitest

import (
	"context"
	"fmt"
	"log"
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
	ma "github.com/multiformats/go-multiaddr"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	billing "github.com/textileio/textile/v2/api/billingd/service"
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
	MakeTextileWithConfig(t, conf)
	return conf
}

func DefaultTextileConfig(t util.TestingTWithCleanup) core.Config {
	apiPort, err := freeport.GetFreePort()
	require.NoError(t, err)
	gatewayPort, err := freeport.GetFreePort()
	require.NoError(t, err)

	return core.Config{
		Hub:                       true,
		Debug:                     true,
		AddrAPI:                   util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", apiPort)),
		AddrAPIProxy:              util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrMongoURI:              GetMongoUri(),
		AddrMongoName:             util.MakeToken(12),
		AddrThreadsHost:           util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrIPFSAPI:               GetIPFSApiAddr(),
		AddrGatewayHost:           util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gatewayPort)),
		AddrGatewayURL:            fmt.Sprintf("http://127.0.0.1:%d", gatewayPort),
		IPNSRepublishSchedule:     "0 1 * * *",
		IPNSRepublishConcurrency:  5,
		CustomerioAPIKey:          os.Getenv("CUSTOMERIO_API_KEY"),
		CustomerioConfirmTmpl:     os.Getenv("CUSTOMERIO_CONFIRM_TMPL"),
		CustomerioInviteTmpl:      os.Getenv("CUSTOMERIO_INVITE_TMPL"),
		EmailSessionSecret:        SessionSecret,
		MaxBucketArchiveRepFactor: 4,
	}
}

type Options struct {
	RepoPath       string
	NoAutoShutdown bool
}

type Option func(*Options)

func WithRepoPath(repoPath string) Option {
	return func(o *Options) {
		o.RepoPath = repoPath
	}
}

func WithoutAutoShutdown() Option {
	return func(o *Options) {
		o.NoAutoShutdown = true
	}
}

func MakeTextileWithConfig(t util.TestingTWithCleanup, conf core.Config, opts ...Option) func() {
	var args Options
	for _, opt := range opts {
		opt(&args)
	}
	if args.RepoPath == "" {
		args.RepoPath = t.TempDir()
	}
	textile, err := core.NewTextile(context.Background(), conf, core.WithBadgerThreadsPersistence(args.RepoPath))
	require.NoError(t, err)
	time.Sleep(5 * time.Second) // Give the api a chance to get ready
	done := func() {
		time.Sleep(time.Second) // Give threads a chance to finish work
		err := textile.Close()
		require.NoError(t, err)
	}
	if !args.NoAutoShutdown {
		t.Cleanup(done)
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
		DBURI:                  GetMongoUri(),
		DBName:                 util.MakeToken(8),
		GatewayHostAddr:        util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gatewayPort)),
		Debug:                  true,
	}
}

func MakeBillingWithConfig(t util.TestingTWithCleanup, conf billing.Config) {
	api, err := billing.NewService(context.Background(), conf)
	require.NoError(t, err)
	err = api.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		err := api.Stop()
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

// GetMongoUri returns env value or default.
func GetMongoUri() string {
	env := os.Getenv("MONGO_URI")
	if env != "" {
		return env
	}
	return "mongodb://127.0.0.1:27017"
}

// GetIPFSApiAddr returns env value or default.
func GetIPFSApiAddr() ma.Multiaddr {
	env := os.Getenv("IPFS_API_ADDR")
	if env != "" {
		return util.MustParseAddr(env)
	}
	return util.MustParseAddr("/ip4/127.0.0.1/tcp/5011")
}

// StartServices starts local mongodb and ipfs services.
func StartServices() (cleanup func()) {
	_, currentFilePath, _, _ := runtime.Caller(0)
	dirpath := path.Dir(currentFilePath)

	makeDown := func() {
		cmd := exec.Command(
			"docker-compose",
			"-f",
			fmt.Sprintf("%s/docker-compose.yml", dirpath),
			"down",
			"-v",
			"--remove-orphans",
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

	limit := 5
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
		makeDown()
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
	mc, err := mongo.Connect(ctx, options.Client().ApplyURI(GetMongoUri()))
	if err != nil {
		return err
	}
	if err = mc.Ping(ctx, nil); err != nil {
		return err
	}
	ic, err := httpapi.NewApi(GetIPFSApiAddr())
	if err != nil {
		return err
	}
	if _, err = ic.Key().Self(ctx); err != nil {
		return err
	}
	return nil
}
