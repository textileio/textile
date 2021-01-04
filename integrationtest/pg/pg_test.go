package pg

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	tutil "github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/apitest"
	c "github.com/textileio/textile/v2/api/bucketsd/client"
	pb "github.com/textileio/textile/v2/api/bucketsd/pb"
	"github.com/textileio/textile/v2/api/common"
	hc "github.com/textileio/textile/v2/api/hubd/client"
	"github.com/textileio/textile/v2/buckets/archive"
	"github.com/textileio/textile/v2/core"
	"github.com/textileio/textile/v2/util"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	archive.CheckInterval = time.Second * 5
	os.Exit(m.Run())
}

func TestCreateBucket(t *testing.T) {
	powc := StartPowergate(t)
	ctx, _, client, _, shutdown := setup(t)
	defer shutdown()

	// User is now created, so it should exist after spinup.
	res, err := powc.Admin.Users.List(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(res.Users))

	_, err = client.Create(ctx)
	require.NoError(t, err)

	// No new user should be created for the bucket.
	res, err = powc.Admin.Users.List(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, len(res.Users))
}

func TestArchiveTracker(t *testing.T) {
	util.RunFlaky(t, func(t *util.FlakyT) {
		_ = StartPowergate(t)
		ctx, conf, client, repo, shutdown := setup(t)

		// Create bucket with a file.
		b, err := client.Create(ctx)
		require.Nil(t, err)
		time.Sleep(4 * time.Second) // Give a sec to fund the Fil address.

		rootCid1 := addDataFileToBucket(ctx, t, client, b.Root.Key, "Data1.txt")

		// Archive it (push to PG)
		err = client.Archive(ctx, b.Root.Key)
		require.NoError(t, err)
		time.Sleep(4 * time.Second) // Give some time to push the archive to PG.

		// Force stop the Hub.
		fmt.Println("<<< Force stopping Hub")
		shutdown()
		fmt.Println("<<< Hub stopped")

		// Re-spin up Hub.
		fmt.Println(">>> Re-spinning the Hub")
		client = reSetup(t, conf, repo)
		time.Sleep(5 * time.Second) // Wait for Hub to spinup and resume archives tracking.
		fmt.Println(">>> Hub started")

		// ## Continue on as nothing "bad" happened and check for success...

		// Wait for the archive to finish.
		require.Eventually(t, archiveFinalState(ctx, t, client, b.Root.Key), 2*time.Minute, 2*time.Second)

		// Verify that the current archive status is Done.
		res, err := client.Archives(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatus_ARCHIVE_STATUS_SUCCESS, res.Current.ArchiveStatus)

		require.Equal(t, rootCid1, res.Current.Cid)
		require.Len(t, res.Current.DealInfo, 1)
		deal := res.Current.DealInfo[0]
		require.NotEmpty(t, deal.ProposalCid)
		require.NotEmpty(t, deal.Miner)
	})
}

func TestArchiveBucketWorkflow(t *testing.T) {
	util.RunFlaky(t, func(t *util.FlakyT) {
		_ = StartPowergate(t)
		ctx, _, client, _, shutdown := setup(t)
		defer shutdown()

		// Create bucket with a file.
		b, err := client.Create(ctx)
		require.NoError(t, err)
		time.Sleep(4 * time.Second) // Give a sec to fund the Fil address.
		rootCid1 := addDataFileToBucket(ctx, t, client, b.Root.Key, "Data1.txt")

		// Archive it (push to PG)
		err = client.Archive(ctx, b.Root.Key)
		require.NoError(t, err)

		// Wait for the archive to finish.
		require.Eventually(t, archiveFinalState(ctx, t, client, b.Root.Key), 2*time.Minute, 2*time.Second)

		// Verify that the current archive status is Done.
		res, err := client.Archives(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatus_ARCHIVE_STATUS_SUCCESS, res.Current.ArchiveStatus, res.Current.FailureMsg)

		require.Equal(t, rootCid1, res.Current.Cid)
		require.Len(t, res.Current.DealInfo, 1)
		deal1 := res.Current.DealInfo[0]
		require.NotEmpty(t, deal1.ProposalCid)
		require.NotEmpty(t, deal1.Miner)

		// List archives.
		as, err := client.ArchivesLs(ctx)
		require.NoError(t, err)
		require.Len(t, as.Archives, 1)
		require.Equal(t, rootCid1, as.Archives[0].Cid)
		require.Len(t, as.Archives[0].Info, 1)
		require.Equal(t, as.Archives[0].Info[0].DealId, deal1.DealId)

		// Add another file to the bucket.
		rootCid2 := addDataFileToBucket(ctx, t, client, b.Root.Key, "Data2.txt")

		// Archive again.
		err = client.Archive(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Eventually(t, archiveFinalState(ctx, t, client, b.Root.Key), 2*time.Minute, 2*time.Second)
		res, err = client.Archives(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatus_ARCHIVE_STATUS_SUCCESS, res.Current.ArchiveStatus)

		require.Equal(t, rootCid2, res.Current.Cid)
		require.Len(t, res.Current.DealInfo, 1)
		deal2 := res.Current.DealInfo[0]
		require.NotEmpty(t, deal2.ProposalCid)
		require.NotEmpty(t, deal2.Miner)

		// List archives.
		as, err = client.ArchivesLs(ctx)
		require.NoError(t, err)
		require.Len(t, as.Archives, 1)
		require.Equal(t, rootCid2, as.Archives[0].Cid)
		require.Len(t, as.Archives[0].Info, 1)
		require.Equal(t, as.Archives[0].Info[0].DealId, deal2.DealId)
	})
}

func TestArchivesLs(t *testing.T) {
	util.RunFlaky(t, func(t *util.FlakyT) {
		_ = StartPowergate(t)
		ctx, _, client, _, shutdown := setup(t)
		defer shutdown()

		// Create Bucket 1, and archive it.
		b, err := client.Create(ctx)
		require.NoError(t, err)
		time.Sleep(4 * time.Second)
		rootCid1 := addDataFileToBucket(ctx, t, client, b.Root.Key, "Data1.txt")
		err = client.Archive(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Eventually(t, archiveFinalState(ctx, t, client, b.Root.Key), 2*time.Minute, 2*time.Second)
		res, err := client.Archives(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatus_ARCHIVE_STATUS_SUCCESS, res.Current.ArchiveStatus, res.Current.FailureMsg)
		deal1 := res.Current.DealInfo[0]

		// List archives, length should be 1.
		as, err := client.ArchivesLs(ctx)
		require.NoError(t, err)
		require.Len(t, as.Archives, 1)
		require.Equal(t, rootCid1, as.Archives[0].Cid)
		require.Len(t, as.Archives[0].Info, 1)
		require.Equal(t, deal1.DealId, as.Archives[0].Info[0].DealId)

		// Create Bucket 2, and archive it.
		b, err = client.Create(ctx)
		require.NoError(t, err)
		time.Sleep(4 * time.Second)
		rootCid2 := addDataFileToBucket(ctx, t, client, b.Root.Key, "Data2.txt")
		err = client.Archive(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Eventually(t, archiveFinalState(ctx, t, client, b.Root.Key), 2*time.Minute, 2*time.Second)
		res, err = client.Archives(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatus_ARCHIVE_STATUS_SUCCESS, res.Current.ArchiveStatus, res.Current.FailureMsg)
		deal2 := res.Current.DealInfo[0]

		// List archives, length should be 2.
		as, err = client.ArchivesLs(ctx)
		require.NoError(t, err)
		require.Len(t, as.Archives, 2)
		sort.Slice(as.Archives, func(i, j int) bool {
			return as.Archives[i].Cid < as.Archives[j].Cid
		})

		// Verify data is correct, notice that the order is flipped
		// due to sort.Slice cids. That's fine.
		require.Equal(t, rootCid2, as.Archives[0].Cid)
		require.Len(t, as.Archives[0].Info, 1)
		require.Equal(t, deal2.DealId, as.Archives[0].Info[0].DealId)
		require.Equal(t, rootCid1, as.Archives[1].Cid)
		require.Len(t, as.Archives[1].Info, 1)
		require.Equal(t, deal1.DealId, as.Archives[1].Info[0].DealId)
	})
}

func TestArchivesImport(t *testing.T) {
	util.RunFlaky(t, func(t *util.FlakyT) {
		_ = StartPowergate(t)
		hubclient, threadsclient, client, conf, _, shutdown := createInfra(t)
		defer shutdown()

		// Create an archive for account-1.
		ctxAccount1 := createAccount(t, hubclient, threadsclient, conf)
		b, err := client.Create(ctxAccount1)
		require.NoError(t, err)
		time.Sleep(4 * time.Second) // Give a sec to fund the Fil address.
		rootCid1 := addDataFileToBucket(ctxAccount1, t, client, b.Root.Key, "Data1.txt")
		cid1, err := cid.Decode(rootCid1)
		require.NoError(t, err)
		err = client.Archive(ctxAccount1, b.Root.Key)
		require.NoError(t, err)
		require.Eventually(t, archiveFinalState(ctxAccount1, t, client, b.Root.Key), 2*time.Minute, 2*time.Second)
		res, err := client.Archives(ctxAccount1, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatus_ARCHIVE_STATUS_SUCCESS, res.Current.ArchiveStatus)
		deal := res.Current.DealInfo[0]

		// Create account-2, and import the DealID
		// created by account-1.
		ctxAccount2 := createAccount(t, hubclient, threadsclient, conf)

		// Check no archives exist for account-2
		as, err := client.ArchivesLs(ctxAccount2)
		require.NoError(t, err)
		require.Len(t, as.Archives, 0)

		// Import deal made by account-1.
		err = client.ArchivesImport(ctxAccount2, cid1, []uint64{deal.DealId})
		require.NoError(t, err)

		// Assert archives listing includes imported archive.
		as, err = client.ArchivesLs(ctxAccount2)
		require.NoError(t, err)
		require.Len(t, as.Archives, 1)

		require.Equal(t, rootCid1, as.Archives[0].Cid)
		require.Len(t, as.Archives[0].Info, 1)
		require.Equal(t, as.Archives[0].Info[0].DealId, deal.DealId)
	})
}

func TestArchiveUnfreeze(t *testing.T) {
	t.Skipf("TODO API")
	util.RunFlaky(t, func(t *util.FlakyT) {
		_ = StartPowergate(t)
		hubclient, threadsclient, client, conf, _, shutdown := createInfra(t)
		defer shutdown()

		// Create an archive for account-1.
		ctxAccount1 := createAccount(t, hubclient, threadsclient, conf)
		buck, err := client.Create(ctxAccount1)
		require.NoError(t, err)
		time.Sleep(4 * time.Second) // Give a sec to fund the Fil address.
		rootCid1 := addDataFileToBucket(ctxAccount1, t, client, buck.Root.Key, "Data1.txt")
		err = client.Archive(ctxAccount1, buck.Root.Key)
		require.NoError(t, err)
		require.Eventually(t, archiveFinalState(ctxAccount1, t, client, buck.Root.Key), 2*time.Minute, 2*time.Second)
		res, err := client.Archives(ctxAccount1, buck.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatus_ARCHIVE_STATUS_SUCCESS, res.Current.ArchiveStatus)
		deal := res.Current.DealInfo[0]
		ccid, err := cid.Decode(rootCid1)
		require.NoError(t, err)

		// Create account-2, and import the DealID
		// created by account-1.
		ctxAccount2 := createAccount(t, hubclient, threadsclient, conf)

		// Obvious assertion about no existing retrievals.
		rs, err := client.ArchiveRetrievalLs(ctxAccount2)
		require.NoError(t, err)
		require.Len(t, len(rs.Archives), 0)

		// Import deal made by account-1.
		err = client.ArchivesImport(ctxAccount2, ccid, []uint64{deal.DealId})
		require.NoError(t, err)

		// Unfreeze to a bucket.
		buck, err = client.Create(ctxAccount2, c.WithCid(ccid), c.WithUnfreeze(true))
		require.NoError(t, err)

		// Wait for the retrieval to finish.
		var r *pb.ArchiveRetrievalLsItem
		require.Eventually(t, func() bool {
			rs, err = client.ArchiveRetrievalLs(ctxAccount2)
			require.NoError(t, err)
			require.Len(t, len(rs.Archives), 1)
			r = rs.Archives[0]

			require.NotEqual(t, pb.ArchiveRetrievalStatus_FAILED, r.Status)

			return r.Status == pb.ArchiveRetrievalStatus_SUCCESS
		}, 2*time.Minute, 2*time.Second)

		// Check if what we got from Filecoin is the same file that
		// account-1 saved in the archived bucket.
		buf := bytes.NewBuffer(nil)
		err = client.PullPath(ctxAccount2, buck.Root.Key, "Data1.txt", buf)
		require.NoError(t, err)
		f, err := os.Open("testdata/Data1.txt")
		require.NoError(t, err)
		t.Cleanup(func() { f.Close() })
		originalFile, err := ioutil.ReadAll(f)
		require.NoError(t, err)
		require.True(t, bytes.Equal(originalFile, buf.Bytes()))

		// Assert there're things in the retrieval log.
		ctxAccount2, cancel := context.WithCancel(ctxAccount2)
		defer cancel()
		ch := make(chan string, 100)
		go func() {
			err = client.ArchiveRetrievalLogs(ctxAccount2, r.Id, ch)
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

func TestArchiveWatch(t *testing.T) {
	util.RunFlaky(t, func(t *util.FlakyT) {
		_ = StartPowergate(t)
		ctx, _, client, _, shutdown := setup(t)
		defer shutdown()

		b, err := client.Create(ctx)
		require.NoError(t, err)
		time.Sleep(4 * time.Second)
		addDataFileToBucket(ctx, t, client, b.Root.Key, "Data1.txt")

		err = client.Archive(ctx, b.Root.Key)
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
		_ = StartPowergate(t)
		ctx, _, client, _, shutdown := setup(t)
		defer shutdown()

		b, err := client.Create(ctx)
		require.NoError(t, err)
		time.Sleep(4 * time.Second)
		// Store a file that is bigger than the sector size, this
		// should lead to an error on the PG side.
		addDataFileToBucket(ctx, t, client, b.Root.Key, "Data3.txt")

		err = client.Archive(ctx, b.Root.Key)
		require.NoError(t, err)

		require.Eventually(t, archiveFinalState(ctx, t, client, b.Root.Key), time.Minute, 2*time.Second)
		res, err := client.Archives(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatus_ARCHIVE_STATUS_FAILED, res.Current.ArchiveStatus)
		require.NotEmpty(t, res.Current.FailureMsg)
	})
}

func archiveFinalState(ctx context.Context, t util.TestingTWithCleanup, client *c.Client, bucketKey string) func() bool {
	return func() bool {
		res, err := client.Archives(ctx, bucketKey)
		require.NoError(t, err)

		if res.Current == nil {
			return false
		}

		switch res.Current.ArchiveStatus {
		case pb.ArchiveStatus_ARCHIVE_STATUS_FAILED,
			pb.ArchiveStatus_ARCHIVE_STATUS_SUCCESS,
			pb.ArchiveStatus_ARCHIVE_STATUS_CANCELED:
			return true
		case pb.ArchiveStatus_ARCHIVE_STATUS_QUEUED:
		case pb.ArchiveStatus_ARCHIVE_STATUS_EXECUTING:
		case pb.ArchiveStatus_ARCHIVE_STATUS_UNSPECIFIED:
		default:
			t.Errorf("unknown archive status %v", pb.ArchiveStatus_name[int32(res.Current.ArchiveStatus)])
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

func setup(t util.TestingTWithCleanup) (context.Context, core.Config, *c.Client, string, func()) {
	hubclient, threadsclient, client, conf, repo, shutdown := createInfra(t)
	ctx := createAccount(t, hubclient, threadsclient, conf)

	return ctx, conf, client, repo, shutdown
}

func createInfra(t util.TestingTWithCleanup) (*hc.Client, *tc.Client, *c.Client, core.Config, string, func()) {
	conf := apitest.DefaultTextileConfig(t)
	conf.AddrPowergateAPI = powAddr
	conf.ArchiveJobPollIntervalFast = time.Second * 5
	conf.ArchiveJobPollIntervalSlow = time.Second * 10
	repo := t.TempDir()
	shutdown := apitest.MakeTextileWithConfig(t, conf, apitest.WithRepoPath(repo), apitest.WithoutAutoShutdown())
	target, err := tutil.TCPAddrFromMultiAddr(conf.AddrAPI)
	require.NoError(t, err)
	opts := []grpc.DialOption{grpc.WithInsecure(), grpc.WithPerRPCCredentials(common.Credentials{})}
	client, err := c.NewClient(target, opts...)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := client.Close()
		require.NoError(t, err)
	})
	hubclient, err := hc.NewClient(target, opts...)
	require.NoError(t, err)
	threadsclient, err := tc.NewClient(target, opts...)
	require.NoError(t, err)

	return hubclient, threadsclient, client, conf, repo, shutdown

}

func createAccount(t util.TestingTWithCleanup, hubclient *hc.Client, threadsclient *tc.Client, conf core.Config) context.Context {
	user := apitest.Signup(t, hubclient, conf, apitest.NewUsername(), apitest.NewEmail())
	ctx := common.NewSessionContext(context.Background(), user.Session)
	id := thread.NewIDV1(thread.Raw, 32)
	ctx = common.NewThreadNameContext(ctx, "buckets")
	err := threadsclient.NewDB(ctx, id)
	require.NoError(t, err)

	return common.NewThreadIDContext(ctx, id)
}

func reSetup(t util.TestingTWithCleanup, conf core.Config, repo string) *c.Client {
	apitest.MakeTextileWithConfig(t, conf, apitest.WithRepoPath(repo))
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
