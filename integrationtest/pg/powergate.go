package pg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"time"

	httpapi "github.com/ipfs/go-ipfs-http-client"
	"github.com/stretchr/testify/require"
	pc "github.com/textileio/powergate/v2/api/client"
	"github.com/textileio/textile/v2/util"
	"google.golang.org/grpc"
)

var powAddr = "127.0.0.1:5002"
var ipfsAddr = "/ip4/127.0.0.1/tcp/5022"

func StartPowergate(t util.TestingTWithCleanup, onlineMode bool) (*pc.Client, *httpapi.HttpApi) {
	os.Setenv("LOTUS_ONLINEMODE", strconv.FormatBool(onlineMode))
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
			t.Errorf("docker-compose down: %s", err)
			t.FailNow()
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
		t.Errorf("docker-compose build: %s", err)
		t.FailNow()
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
		t.Errorf("running docker-compose: %s", err)
		t.FailNow()
	}
	t.Cleanup(makeDown)

	var powc *pc.Client
	var err error
	limit := 100
	retries := 0
	require.Nil(t, err)
	for retries < limit {
		powc, err = pc.NewClient(powAddr, grpc.WithInsecure())
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		_, err = powc.BuildInfo(ctx)
		if err == nil {
			cancel()
			break
		}
		time.Sleep(time.Second)
		retries++
		cancel()
	}
	if retries == limit {
		if err != nil {
			t.Errorf("trying to confirm build info: %s", err)
			t.FailNow()
		}
		t.Errorf("max retries to connect with Powergate")
		t.FailNow()
	}

	ia := util.MustParseAddr(ipfsAddr)
	ipfs, err := httpapi.NewApi(ia)
	require.NoError(t, err)

	return powc, ipfs
}
