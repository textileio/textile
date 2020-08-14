package pg

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	pc "github.com/textileio/powergate/api/client"
	"github.com/textileio/powergate/health"
	"github.com/textileio/textile/api/apitest"
	c "github.com/textileio/textile/api/buckets/client"
	pb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/api/common"
	hc "github.com/textileio/textile/api/hub/client"
	"github.com/textileio/textile/buckets/archive"
	"github.com/textileio/textile/core"
	"github.com/textileio/textile/util"
	"google.golang.org/grpc"
)

var powAddr = "127.0.0.1:5002"

func TestMain(m *testing.M) {
	archive.CheckInterval = time.Second * 5
	archive.JobStatusPollInterval = time.Second * 5
	os.Exit(m.Run())
}

func TestCreateBucket(t *testing.T) {
	powc := spinup(t)
	ctx, _, client, shutdown := setup(t)
	defer shutdown(true)

	// FFS is now created for the user, so it should exist after spinup
	lst, err := powc.FFS.ListAPI(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(lst))

	_, err = client.Init(ctx)
	require.NoError(t, err)

	// No new FFS instance should be created for the bucket
	lst, err = powc.FFS.ListAPI(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(lst))
}

func TestArchiveTracker(t *testing.T) {
	util.RunFlaky(t, func(t *util.FlakyT) {
		_ = spinup(t)
		ctx, conf, client, shutdown := setup(t)

		// Create bucket with a file.
		b, err := client.Init(ctx)
		require.Nil(t, err)
		time.Sleep(4 * time.Second) // Give a sec to fund the Fil address.

		rootCid1 := addDataFileToBucket(ctx, t, client, b.Root.Key, "Data1.txt")

		// Archive it (push to PG)
		_, err = client.Archive(ctx, b.Root.Key)
		require.NoError(t, err)
		time.Sleep(4 * time.Second) // Give some time to push the archive to PG.

		// Force stop the Hub.
		fmt.Println("<<< Force stopping Hub")
		shutdown(false)
		fmt.Println("<<< Hub stopped")

		// Re-spin up Hub.
		fmt.Println(">>> Re-spinning the Hub")
		client = reSetup(t, conf)
		time.Sleep(5 * time.Second) // Wait for Hub to spinup and resume archives tracking.
		fmt.Println(">>> Hub started")

		// ## Continue on as nothing "bad" happened and check for success...

		// Wait for the archive to finish.
		require.Eventually(t, archiveFinalState(ctx, t, client, b.Root.Key), 120*time.Second, 2*time.Second)

		// Verify that the current archive status is Done.
		as, err := client.ArchiveStatus(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatusReply_Done, as.GetStatus())

		// Get ArchiveInfo, which has all successful pushs with
		// its data about deals.
		ai, err := client.ArchiveInfo(ctx, b.Root.Key)
		require.NoError(t, err)

		archive := ai.GetArchive()
		require.Equal(t, rootCid1, archive.Cid)
		require.Len(t, archive.Deals, 1)
		deal := archive.Deals[0]
		require.NotEmpty(t, deal.GetProposalCid())
		require.NotEmpty(t, deal.GetMiner())
	})
}

func TestArchiveBucketWorkflow(t *testing.T) {
	util.RunFlaky(t, func(t *util.FlakyT) {
		_ = spinup(t)
		ctx, _, client, shutdown := setup(t)
		defer shutdown(true)

		// Create bucket with a file.
		b, err := client.Init(ctx)
		require.NoError(t, err)
		time.Sleep(4 * time.Second) // Give a sec to fund the Fil address.
		rootCid1 := addDataFileToBucket(ctx, t, client, b.Root.Key, "Data1.txt")

		// Archive it (push to PG)
		_, err = client.Archive(ctx, b.Root.Key)
		require.NoError(t, err)

		// Wait for the archive to finish.
		require.Eventually(t, archiveFinalState(ctx, t, client, b.Root.Key), 120*time.Second, 2*time.Second)

		// Verify that the current archive status is Done.
		as, err := client.ArchiveStatus(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatusReply_Done, as.GetStatus())

		// Get ArchiveInfo, which has all successful pushs with
		// its data about deals.
		ai, err := client.ArchiveInfo(ctx, b.Root.Key)
		require.NoError(t, err)

		archive := ai.GetArchive()
		require.Equal(t, rootCid1, archive.Cid)
		require.Len(t, archive.Deals, 1)
		deal := archive.Deals[0]
		require.NotEmpty(t, deal.GetProposalCid())
		require.NotEmpty(t, deal.GetMiner())

		// Add another file to the bucket.
		rootCid2 := addDataFileToBucket(ctx, t, client, b.Root.Key, "Data2.txt")

		// Archive again.
		_, err = client.Archive(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Eventually(t, archiveFinalState(ctx, t, client, b.Root.Key), 120*time.Second, 2*time.Second)
		as, err = client.ArchiveStatus(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatusReply_Done, as.GetStatus())

		ai, err = client.ArchiveInfo(ctx, b.Root.Key)
		require.NoError(t, err)

		archive = ai.GetArchive()
		require.Equal(t, rootCid2, archive.Cid)
		require.Len(t, archive.Deals, 1)
		deal = archive.Deals[0]
		require.NotEmpty(t, deal.GetProposalCid())
		require.NotEmpty(t, deal.GetMiner())
	})
}

func TestArchiveWatch(t *testing.T) {
	util.RunFlaky(t, func(t *util.FlakyT) {
		_ = spinup(t)
		ctx, _, client, shutdown := setup(t)
		defer shutdown(true)

		b, err := client.Init(ctx)
		require.NoError(t, err)
		time.Sleep(4 * time.Second)
		addDataFileToBucket(ctx, t, client, b.Root.Key, "Data1.txt")

		_, err = client.Archive(ctx, b.Root.Key)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		ch := make(chan string, 100)
		go func() {
			err = client.ArchiveWatch(ctx, b.Root.Key, ch)
			close(ch)
		}()
		count := 0
		for s := range ch {
			require.NotEmpty(t, s)
			count++
			if count > 4 {
				cancel()
			}
		}
		require.NoError(t, err)
		require.Greater(t, count, 3)
	})
}

func TestFailingArchive(t *testing.T) {
	util.RunFlaky(t, func(t *util.FlakyT) {
		_ = spinup(t)
		ctx, _, client, shutdown := setup(t)
		defer shutdown(true)

		b, err := client.Init(ctx)
		require.NoError(t, err)
		time.Sleep(4 * time.Second)
		// Store a file that is bigger than the sector size, this
		// should lead to an error on the PG side.
		addDataFileToBucket(ctx, t, client, b.Root.Key, "Data3.txt")

		_, err = client.Archive(ctx, b.Root.Key)
		require.NoError(t, err)

		require.Eventually(t, archiveFinalState(ctx, t, client, b.Root.Key), 60*time.Second, 2*time.Second)
		as, err := client.ArchiveStatus(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatusReply_Failed, as.GetStatus())
		require.NotEmpty(t, as.GetFailedMsg())
	})
}

func archiveFinalState(ctx context.Context, t util.TestingTWithCleanup, client *c.Client, bucketKey string) func() bool {
	return func() bool {
		as, err := client.ArchiveStatus(ctx, bucketKey)
		require.NoError(t, err)

		switch as.GetStatus() {
		case pb.ArchiveStatusReply_Failed,
			pb.ArchiveStatusReply_Done,
			pb.ArchiveStatusReply_Canceled:
			return true
		case pb.ArchiveStatusReply_Executing:
		default:
			t.Errorf("unknown archive status")
			t.FailNow()
		}
		return false
	}
}

// addDataFileToBucket add a file from the testdata folder, and returns the
// new stringified root Cid of the bucket.
func addDataFileToBucket(ctx context.Context, t util.TestingTWithCleanup, client *c.Client, bucketKey string, fileName string) string {
	f, err := os.Open("testdata/" + fileName)
	require.NoError(t, err)
	t.Cleanup(func() { f.Close() })

	pth, root, err := client.PushPath(ctx, bucketKey, fileName, f)
	require.NoError(t, err)
	assert.NotEmpty(t, pth)
	assert.NotEmpty(t, root)

	return strings.SplitN(root.String(), "/", 4)[2]
}

func setup(t util.TestingTWithCleanup) (context.Context, core.Config, *c.Client, func(bool)) {
	conf := apitest.DefaultTextileConfig(t)
	conf.AddrPowergateAPI = powAddr
	conf.AddrIPFSAPI = util.MustParseAddr("/ip4/127.0.0.1/tcp/5011")
	conf.AddrMongoURI = "mongodb://127.0.0.1:27027"
	shutdown := apitest.MakeTextileWithConfig(t, conf, false)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.NoError(t, err)
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{})}
	client, err := c.NewClient(target, opts...)
	require.NoError(t, err)
	hubclient, err := hc.NewClient(target, opts...)
	require.NoError(t, err)
	threadsclient, err := tc.NewClient(target, opts...)
	require.NoError(t, err)

	user := apitest.Signup(t, hubclient, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	id := thread.NewIDV1(thread.Raw, 32)
	ctx = common.NewThreadNameContext(ctx, "buckets")
	err = threadsclient.NewDB(ctx, id)
	require.NoError(t, err)
	ctx = common.NewThreadIDContext(ctx, id)
	t.Cleanup(func() {
		err := client.Close()
		require.NoError(t, err)
	})

	return ctx, conf, client, shutdown
}

func reSetup(t util.TestingTWithCleanup, conf core.Config) *c.Client {
	apitest.MakeTextileWithConfig(t, conf, true)
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.Nil(t, err)
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{})}
	client, err := c.NewClient(target, opts...)
	require.Nil(t, err)

	t.Cleanup(func() {
		err := client.Close()
		require.Nil(t, err)
	})

	return client
}

func spinup(t util.TestingTWithCleanup) *pc.Client {
	makeDown := func() {
		if err := exec.Command("docker-compose", "down", "-v").Run(); err != nil {
			panic(err)
		}
	}
	makeDown()

	cmd := exec.Command("docker-compose", "build")
	//cmd.Stdout = os.Stdout
	//cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Errorf("docker-compose build: %s", err)
		t.FailNow()
	}

	cmd = exec.Command("docker-compose", "up", "-V")
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
