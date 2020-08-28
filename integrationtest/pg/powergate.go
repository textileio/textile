package pg

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"runtime"
	"time"

	"github.com/stretchr/testify/require"
	pc "github.com/textileio/powergate/api/client"
	"github.com/textileio/powergate/health"
	"github.com/textileio/textile/util"
	"google.golang.org/grpc"
)

var powAddr = "127.0.0.1:5002"

func StartPowergate(t util.TestingTWithCleanup) *pc.Client {
	_, currentFilePath, _, _ := runtime.Caller(0)
	dirpath := path.Dir(currentFilePath)

	makeDown := func() {
		if err := exec.Command("docker-compose", "-f", fmt.Sprintf("%s/docker-compose.yml", dirpath), "down", "-v").Run(); err != nil {
			panic(err)
		}
	}
	makeDown()

	cmd := exec.Command("docker-compose", "-f", fmt.Sprintf("%s/docker-compose.yml", dirpath), "build")
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("docker-compose build: %s", err)
		t.FailNow()
	}

	cmd = exec.Command("docker-compose", "-f", fmt.Sprintf("%s/docker-compose.yml", dirpath), "up", "-V")
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Errorf("running docker-compose: %s", err)
		t.FailNow()
	}
	t.Cleanup(makeDown)

	var powc *pc.Client
	var err error
	limit := 30
	retries := 0
	require.Nil(t, err)
	for retries < limit {
		powc, err = pc.NewClient(powAddr, grpc.WithInsecure())
		require.NoError(t, err)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		s, _, err := powc.Health.Check(ctx)
		if err == nil {
			require.Equal(t, health.Ok, s)
			cancel()
			break
		}
		time.Sleep(time.Second)
		retries++
		cancel()
	}
	if retries == limit {
		if err != nil {
			t.Errorf("trying to confirm health check: %s", err)
			t.FailNow()
		}
		t.Errorf("max retries to connect with Powergate")
		t.FailNow()
	}
	return powc
}
