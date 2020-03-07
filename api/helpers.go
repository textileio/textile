package api

import (
	"context"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
	"github.com/textileio/textile/core"
	"github.com/textileio/textile/util"
)

var TestSessionSecret = uuid.New().String()

func MakeTestTextile(t *testing.T) (conf core.Config, shutdown func()) {
	time.Sleep(time.Second * time.Duration(rand.Intn(5)))

	dir, err := ioutil.TempDir("", "")
	require.Nil(t, err)

	apiPort, err := freeport.GetFreePort()
	require.Nil(t, err)
	gatewayPort, err := freeport.GetFreePort()
	require.Nil(t, err)
	threadsServiceApiPort, err := freeport.GetFreePort()
	require.Nil(t, err)
	threadsApiPort, err := freeport.GetFreePort()
	require.Nil(t, err)

	conf = core.Config{
		RepoPath: dir,

		AddrApi:      util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", apiPort)),
		AddrApiProxy: util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),

		AddrThreadsHost:            util.MustParseAddr("/ip4/0.0.0.0/tcp/0"),
		AddrThreadsServiceApi:      util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", threadsServiceApiPort)),
		AddrThreadsServiceApiProxy: util.MustParseAddr("/ip4/127.0.0.1/tcp/0"),
		AddrThreadsApi:             util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", threadsApiPort)),
		AddrThreadsApiProxy:        util.MustParseAddr("/ip4/127.0.0.1/tcp/0"),

		AddrIpfsApi: util.MustParseAddr("/ip4/127.0.0.1/tcp/5001"),

		AddrGatewayHost: util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", gatewayPort)),
		AddrGatewayUrl:  fmt.Sprintf("http://127.0.0.1:%d", gatewayPort),

		EmailFrom:   "test@email.textile.io",
		EmailDomain: "email.textile.io",
		EmailApiKey: "",

		SessionSecret: TestSessionSecret,

		Debug: true,
	}
	textile, err := core.NewTextile(context.Background(), conf)
	require.Nil(t, err)
	textile.Bootstrap()

	return conf, func() {
		time.Sleep(time.Second) // give threads a chance to finish work
		textile.Close()
		_ = os.RemoveAll(dir)
	}
}
