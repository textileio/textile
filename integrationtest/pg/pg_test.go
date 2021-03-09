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
	httpapi "github.com/ipfs/go-ipfs-http-client"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
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
	uc "github.com/textileio/textile/v2/api/usersd/client"
	userspb "github.com/textileio/textile/v2/api/usersd/pb"
	"github.com/textileio/textile/v2/buckets/archive/retrieval"
	"github.com/textileio/textile/v2/buckets/archive/tracker"
	"github.com/textileio/textile/v2/core"
	"github.com/textileio/textile/v2/util"
	"google.golang.org/grpc"
)

func TestMain(m *testing.M) {
	retrieval.CITest = true
	tracker.CheckInterval = time.Second * 5
	os.Exit(m.Run())
}

func TestCreateBucket(t *testing.T) {
	powc, _ := StartPowergate(t, true)
	ctx, _, _, _, client, _, shutdown := setup(t)
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
		_, _ = StartPowergate(t, true)
		ctx, conf, _, _, client, repo, shutdown := setup(t)

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
		_, _ = StartPowergate(t, true)
		ctx, _, _, usersClient, buckClient, _, shutdown := setup(t)
		defer shutdown()

		// Create bucket with a file.
		b, err := buckClient.Create(ctx)
		require.NoError(t, err)
		time.Sleep(4 * time.Second) // Give a sec to fund the Fil address.
		rootCid1 := addDataFileToBucket(ctx, t, buckClient, b.Root.Key, "Data1.txt")

		// Archive it (push to PG)
		err = buckClient.Archive(ctx, b.Root.Key)
		require.NoError(t, err)

		// Wait for the archive to finish.
		require.Eventually(t, archiveFinalState(ctx, t, buckClient, b.Root.Key), 2*time.Minute, 2*time.Second)

		// Verify that the current archive status is Done.
		res, err := buckClient.Archives(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatus_ARCHIVE_STATUS_SUCCESS, res.Current.ArchiveStatus, res.Current.FailureMsg)

		require.Equal(t, rootCid1, res.Current.Cid)
		require.Len(t, res.Current.DealInfo, 1)
		deal1 := res.Current.DealInfo[0]
		require.NotEmpty(t, deal1.ProposalCid)
		require.NotEmpty(t, deal1.Miner)

		// List archives.
		as, err := usersClient.ArchivesLs(ctx)
		require.NoError(t, err)
		require.Len(t, as.Archives, 1)
		require.Equal(t, rootCid1, as.Archives[0].Cid)
		require.Len(t, as.Archives[0].Info, 1)
		require.Equal(t, as.Archives[0].Info[0].DealId, deal1.DealId)

		// Add another file to the bucket.
		rootCid2 := addDataFileToBucket(ctx, t, buckClient, b.Root.Key, "Data2.txt")

		// Archive again.
		err = buckClient.Archive(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Eventually(t, archiveFinalState(ctx, t, buckClient, b.Root.Key), 2*time.Minute, 2*time.Second)
		res, err = buckClient.Archives(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatus_ARCHIVE_STATUS_SUCCESS, res.Current.ArchiveStatus)

		require.Equal(t, rootCid2, res.Current.Cid)
		require.Len(t, res.Current.DealInfo, 1)
		deal2 := res.Current.DealInfo[0]
		require.NotEmpty(t, deal2.ProposalCid)
		require.NotEmpty(t, deal2.Miner)

		// List archives.
		as, err = usersClient.ArchivesLs(ctx)
		require.NoError(t, err)
		require.Len(t, as.Archives, 1)
		require.Equal(t, rootCid2, as.Archives[0].Cid)
		require.Len(t, as.Archives[0].Info, 1)
		require.Equal(t, as.Archives[0].Info[0].DealId, deal2.DealId)
	})
}

func TestArchivesLs(t *testing.T) {
	util.RunFlaky(t, func(t *util.FlakyT) {
		_, _ = StartPowergate(t, true)
		ctx, _, _, usersClient, buckClient, _, shutdown := setup(t)
		defer shutdown()

		// Create Bucket 1, and archive it.
		b, err := buckClient.Create(ctx)
		require.NoError(t, err)
		time.Sleep(4 * time.Second)
		rootCid1 := addDataFileToBucket(ctx, t, buckClient, b.Root.Key, "Data1.txt")
		err = buckClient.Archive(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Eventually(t, archiveFinalState(ctx, t, buckClient, b.Root.Key), 2*time.Minute, 2*time.Second)
		res, err := buckClient.Archives(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatus_ARCHIVE_STATUS_SUCCESS, res.Current.ArchiveStatus, res.Current.FailureMsg)
		deal1 := res.Current.DealInfo[0]

		// List archives, length should be 1.
		as, err := usersClient.ArchivesLs(ctx)
		require.NoError(t, err)
		require.Len(t, as.Archives, 1)
		require.Equal(t, rootCid1, as.Archives[0].Cid)
		require.Len(t, as.Archives[0].Info, 1)
		require.Equal(t, deal1.DealId, as.Archives[0].Info[0].DealId)

		// Create Bucket 2, and archive it.
		b, err = buckClient.Create(ctx)
		require.NoError(t, err)
		time.Sleep(4 * time.Second)
		rootCid2 := addDataFileToBucket(ctx, t, buckClient, b.Root.Key, "Data2.txt")
		err = buckClient.Archive(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Eventually(t, archiveFinalState(ctx, t, buckClient, b.Root.Key), 2*time.Minute, 2*time.Second)
		res, err = buckClient.Archives(ctx, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatus_ARCHIVE_STATUS_SUCCESS, res.Current.ArchiveStatus, res.Current.FailureMsg)
		deal2 := res.Current.DealInfo[0]

		// List archives, length should be 2.
		as, err = usersClient.ArchivesLs(ctx)
		require.NoError(t, err)
		require.Len(t, as.Archives, 2)
		sort.Slice(as.Archives, func(i, j int) bool {
			return as.Archives[i].Cid < as.Archives[j].Cid
		})
		cids := []struct {
			c      string
			dealID uint64
		}{
			{c: rootCid1, dealID: deal1.DealId},
			{c: rootCid2, dealID: deal2.DealId},
		}
		sort.Slice(cids, func(i, j int) bool {
			return cids[i].c < cids[j].c
		})

		require.Equal(t, cids[0].c, as.Archives[0].Cid)
		require.Len(t, as.Archives[0].Info, 1)
		require.Equal(t, cids[0].dealID, as.Archives[0].Info[0].DealId)
		require.Equal(t, cids[1].c, as.Archives[1].Cid)
		require.Len(t, as.Archives[1].Info, 1)
		require.Equal(t, cids[1].dealID, as.Archives[1].Info[0].DealId)
	})
}

func TestArchivesImport(t *testing.T) {
	util.RunFlaky(t, func(t *util.FlakyT) {
		_, _ = StartPowergate(t, true)
		hubClient, usersClient, threadsclient, buckClient, conf, _, shutdown := createInfra(t)
		defer shutdown()

		// Create an archive for account-1.
		ctxAccount1 := createAccount(t, hubClient, threadsclient, conf)
		b, err := buckClient.Create(ctxAccount1)
		require.NoError(t, err)
		time.Sleep(4 * time.Second) // Give a sec to fund the Fil address.
		rootCid1 := addDataFileToBucket(ctxAccount1, t, buckClient, b.Root.Key, "Data1.txt")
		cid1, err := cid.Decode(rootCid1)
		require.NoError(t, err)
		err = buckClient.Archive(ctxAccount1, b.Root.Key)
		require.NoError(t, err)
		require.Eventually(t, archiveFinalState(ctxAccount1, t, buckClient, b.Root.Key), 2*time.Minute, 2*time.Second)
		res, err := buckClient.Archives(ctxAccount1, b.Root.Key)
		require.NoError(t, err)
		require.Equal(t, pb.ArchiveStatus_ARCHIVE_STATUS_SUCCESS, res.Current.ArchiveStatus)
		deal := res.Current.DealInfo[0]

		// Create account-2, and import the DealID
		// created by account-1.
		ctxAccount2 := createAccount(t, hubClient, threadsclient, conf)

		// Check no archives exist for account-2
		as, err := usersClient.ArchivesLs(ctxAccount2)
		require.NoError(t, err)
		require.Len(t, as.Archives, 0)

		// Import deal made by account-1.
		err = usersClient.ArchivesImport(ctxAccount2, cid1, []uint64{deal.DealId})
		require.NoError(t, err)

		// Assert archives listing includes imported archive.
		as, err = usersClient.ArchivesLs(ctxAccount2)
		require.NoError(t, err)
		require.Len(t, as.Archives, 1)

		require.Equal(t, rootCid1, as.Archives[0].Cid)
		require.Len(t, as.Archives[0].Info, 1)
		require.Equal(t, as.Archives[0].Info[0].DealId, deal.DealId)
	})
}

func TestArchiveUnfreeze(t *testing.T) {
	util.RunFlaky(t, func(t *util.FlakyT) {
		_, ipfsPow := StartPowergate(t, false)
		hubClient, usersClient, threadsclient, client, conf, _, shutdown := createInfra(t)
		defer shutdown()
		// Create an archive for account-1.
		ctxAccount1 := createAccount(t, hubClient, threadsclient, conf)
		buck, err := client.Create(ctxAccount1)
		require.NoError(t, err)
		time.Sleep(4 * time.Second) // Give a sec to fund the Fil address.
		rootCid1 := addDataFileToBucket(ctxAccount1, t, client, buck.Root.Key, "Data1.txt")

		// <LOTUS-BUG-HACK>
		ccid2, _ := cid.Decode(rootCid1)
		err = ipfsPow.Pin().Add(context.Background(), path.IpldPath(ccid2))
		// </LOTUS-BUG-HACK>

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
		ctxAccount2 := createAccount(t, hubClient, threadsclient, conf)

		// Obvious assertion about no existing retrievals.
		rs, err := usersClient.ArchiveRetrievalLs(ctxAccount2)
		require.NoError(t, err)
		require.Len(t, rs.Retrievals, 0)
		// Import deal made by account-1.
		err = usersClient.ArchivesImport(ctxAccount2, ccid, []uint64{deal.DealId})
		require.NoError(t, err)

		// Delete `ccid` from go-ipfs of Powergate, and go-ipfs of Hub.
		// We need to do this to be sure that we're unfreezing from Filecoin,
		// and not leveraging that the Cid is already in the 'network'.
		//
		// In the real world, if people unfreeze archived data from the Hub, then
		// most probably Powergate will leverage that the data is still available
		// in the Hub. But in this test we want to *force* a real unfreeze, we
		// we'll delete now the archive bucket data from both go-ipfs nodes (Hub's
		// and Powergate).
		ctx := context.Background()

		// <LOTUS-BUG-HACK>
		err = ipfsPow.Pin().Rm(context.Background(), path.IpldPath(ccid2), options.Pin.RmRecursive(true))
		// </LOTUS-BUG-HACK>

		err = ipfsPow.Dag().Remove(context.Background(), ccid)
		require.NoError(t, err)

		// Delete the bucket data from Hub's go-ipfs, basically
		// force deleting the bucket. This is to avoid that go-ipfs from
		// Powergate finds the data since both are conneted.
		// We have to do an extra .Pin().Rm() call since you can't
		// simply remove pinned data without an error.
		ipfsHub, err := httpapi.NewApi(conf.AddrIPFSAPI)
		require.NoError(t, err)
		err = ipfsHub.Pin().Rm(ctx, path.IpldPath(ccid), options.Pin.RmRecursive(true))
		require.NoError(t, err)
		err = ipfsHub.Dag().Remove(context.Background(), ccid)
		require.NoError(t, err)

		// At this point, the archived bucket data was remove from
		// both go-ipfs nodes; so the only way to get this data back if it
		// comes from the miners sector.
		// This was verified anyway. If you change the code below to see
		// Powergate logs, you can doble-check this is the case (since Powergate
		// logs explain from where the data was retrieved).
		// Unfreeze to a bucket.

		buck, err = client.Create(ctxAccount2, c.WithName("super-bucket"), c.WithCid(ccid), c.WithUnfreeze(true))
		require.NoError(t, err)
		// Wait for the retrieval to finish.
		var r *userspb.ArchiveRetrievalLsItem
		require.Eventually(t, func() bool {
			rs, err = usersClient.ArchiveRetrievalLs(ctxAccount2)
			require.NoError(t, err)
			require.Len(t, rs.Retrievals, 1)
			r = rs.Retrievals[0]
			require.NotEqual(t, userspb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_FAILED, r.Status)
			return r.Status == userspb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_SUCCESS
		}, 2*time.Minute, 2*time.Second)
		// Check if what we got from Filecoin is the same file that
		// account-1 saved in the archived bucket.
		lr, err := client.List(ctxAccount2)
		require.NoError(t, err)
		require.Len(t, lr.Roots, 1)
		require.Equal(t, "super-bucket", lr.Roots[0].Name)
		buf := bytes.NewBuffer(nil)
		err = client.PullPath(ctxAccount2, lr.Roots[0].Key, "Data1.txt", buf)
		require.NoError(t, err)
		f, err := os.Open("testdata/Data1.txt")
		require.NoError(t, err)
		t.Cleanup(func() { f.Close() })
		originalFile, err := ioutil.ReadAll(f)
		require.NoError(t, err)
		require.True(t, bytes.Equal(originalFile, buf.Bytes()))

		// Assert there're messages in the retrieval log.
		ctxAccount2, cancel := context.WithCancel(ctxAccount2)
		defer cancel()
		ch := make(chan string, 100)
		go func() {
			err = usersClient.ArchiveRetrievalLogs(ctxAccount2, r.Id, ch)
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
		_, _ = StartPowergate(t, true)
		ctx, _, _, _, client, _, shutdown := setup(t)
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
		_, _ = StartPowergate(t, true)
		ctx, _, _, _, client, _, shutdown := setup(t)
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
	q, err := client.PushPaths(ctx, bucketKey)
	require.NoError(t, err)
	err = q.AddFile(fileName, "testdata/"+fileName)
	require.NoError(t, err)
	for q.Next() {
		require.NoError(t, q.Err())
		assert.NotEmpty(t, q.Current.Path)
		assert.NotEmpty(t, q.Current.Root)
	}
	q.Close()
	return strings.SplitN(q.Current.Root.String(), "/", 4)[2]
}

func setup(t util.TestingTWithCleanup) (context.Context, core.Config, *hc.Client, *uc.Client, *c.Client, string, func()) {
	hubClient, usersClient, threadsclient, buckClient, conf, repo, shutdown := createInfra(t)
	ctx := createAccount(t, hubClient, threadsclient, conf)

	return ctx, conf, hubClient, usersClient, buckClient, repo, shutdown
}

func createInfra(t util.TestingTWithCleanup) (*hc.Client, *uc.Client, *tc.Client, *c.Client, core.Config, string, func()) {
	conf := apitest.DefaultTextileConfig(t)
	conf.AddrPowergateAPI = powAddr
	conf.ArchiveJobPollIntervalFast = time.Second * 5
	conf.ArchiveJobPollIntervalSlow = time.Second * 10
	conf.MinBucketArchiveSize = 0
	conf.MaxBucketArchiveSize = 500 * 1024 * 1024
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
	usersclient, err := uc.NewClient(target, opts...)
	require.NoError(t, err)
	threadsclient, err := tc.NewClient(target, opts...)
	require.NoError(t, err)

	return hubclient, usersclient, threadsclient, client, conf, repo, shutdown

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
