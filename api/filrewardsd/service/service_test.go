package service

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-ds-mongo/test"
	analyticspb "github.com/textileio/textile/v2/api/analyticsd/pb"
	pb "github.com/textileio/textile/v2/api/filrewardsd/pb"
	"github.com/textileio/textile/v2/util"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const bufSize = 1024 * 1024

var (
	ctx, _ = context.WithTimeout(context.Background(), 1*time.Minute)
)

func TestMain(m *testing.M) {
	cleanup := func() {}
	if os.Getenv("SKIP_SERVICES") != "true" {
		cleanup = test.StartMongoDB()
	}
	exitVal := m.Run()
	cleanup()
	os.Exit(exitVal)
}

func TestProcessEvent(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	r := requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	require.Equal(t, "org1", r.OrgKey)
	require.Equal(t, "user1", r.DevKey)
	require.Equal(t, pb.RewardType_REWARD_TYPE_INITIAL_BILLING_SETUP, r.Type)
}

func TestProcessDuplicateOrgUserEvent(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireNoProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
}

func TestProcessDuplicateOrgEvent(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireNoProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
}

func TestProcessDuplicateUserEvent(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireNoProcessedEvent(t, ctx, c, "org2", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
}

func TestProcessDuplicateEvent(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
}

func TestProcessDuplicateOrgUser(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
}

func TestDuplicateFromInitializedCache(t *testing.T) {
	t.Parallel()

	listener1 := bufconn.Listen(bufSize)

	bufDialer1 := func(context.Context, string) (net.Conn, error) {
		return listener1.Dial()
	}

	conf1 := Config{
		Listener:    listener1,
		MongoUri:    test.GetMongoUri(),
		MongoDbName: "mydb",
	}
	s1, err := New(ctx, conf1)
	require.NoError(t, err)

	conn1, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer1), grpc.WithInsecure())
	require.NoError(t, err)
	c1 := pb.NewFilRewardsServiceClient(conn1)

	defer func() {
		conn1.Close()
		s1.Close()
	}()

	requireProcessedEvent(t, ctx, c1, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c1, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c1, "org2", "user2", analyticspb.Event_EVENT_BILLING_SETUP)

	listener2 := bufconn.Listen(bufSize)

	bufDialer2 := func(context.Context, string) (net.Conn, error) {
		return listener2.Dial()
	}

	conf2 := Config{
		Listener:    listener2,
		MongoUri:    test.GetMongoUri(),
		MongoDbName: "mydb",
	}
	s2, err := New(ctx, conf2)
	require.NoError(t, err)

	conn2, err := grpc.DialContext(ctx, "bufnet", grpc.WithContextDialer(bufDialer2), grpc.WithInsecure())
	require.NoError(t, err)
	c2 := pb.NewFilRewardsServiceClient(conn1)

	defer func() {
		conn2.Close()
		s2.Close()
	}()

	require.NoError(t, err)
	requireNoProcessedEvent(t, ctx, c2, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireNoProcessedEvent(t, ctx, c2, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireNoProcessedEvent(t, ctx, c2, "org2", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
}

func TestListRewards(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	res, err := c.ListRewards(ctx, &pb.ListRewardsRequest{})
	require.NoError(t, err)
	require.Len(t, res.Rewards, 3)
}

func TestListRewardsOrgKeyFilter(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	res, err := c.ListRewards(ctx, &pb.ListRewardsRequest{OrgKeyFilter: "org1"})
	require.NoError(t, err)
	require.Len(t, res.Rewards, 2)
}

func TestListRewardsDevKeyFilter(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	res, err := c.ListRewards(ctx, &pb.ListRewardsRequest{DevKeyFilter: "user1"})
	require.NoError(t, err)
	require.Len(t, res.Rewards, 2)
}

func TestListRewardsEventFilter(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	res, err := c.ListRewards(ctx, &pb.ListRewardsRequest{RewardTypeFilter: pb.RewardType_REWARD_TYPE_INITIAL_BILLING_SETUP})
	require.NoError(t, err)
	require.Len(t, res.Rewards, 2)
}

func TestListRewardsOrgKeyAndEventFilters(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	res, err := c.ListRewards(ctx, &pb.ListRewardsRequest{OrgKeyFilter: "org1", RewardTypeFilter: pb.RewardType_REWARD_TYPE_INITIAL_BILLING_SETUP})
	require.NoError(t, err)
	require.Len(t, res.Rewards, 1)
}

func TestListRewardsDevKeyAndEventFilters(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	res, err := c.ListRewards(ctx, &pb.ListRewardsRequest{DevKeyFilter: "user2", RewardTypeFilter: pb.RewardType_REWARD_TYPE_INITIAL_BILLING_SETUP})
	require.NoError(t, err)
	require.Len(t, res.Rewards, 1)
}

func TestListRewardsOrgKeyAndDevKeyAndEventFilters(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	res, err := c.ListRewards(ctx, &pb.ListRewardsRequest{OrgKeyFilter: "org2", DevKeyFilter: "user2", RewardTypeFilter: pb.RewardType_REWARD_TYPE_INITIAL_BILLING_SETUP})
	require.NoError(t, err)
	require.Len(t, res.Rewards, 1)
}

func TestListNonMatchingFilters(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	res, err := c.ListRewards(ctx, &pb.ListRewardsRequest{DevKeyFilter: "user3", RewardTypeFilter: pb.RewardType_REWARD_TYPE_INITIAL_BILLING_SETUP})
	require.NoError(t, err)
	require.Len(t, res.Rewards, 0)
}

func TestListRewardsNoData(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	res, err := c.ListRewards(ctx, &pb.ListRewardsRequest{DevKeyFilter: "user2", RewardTypeFilter: pb.RewardType_REWARD_TYPE_INITIAL_BILLING_SETUP})
	require.NoError(t, err)
	require.Len(t, res.Rewards, 0)
	res, err = c.ListRewards(ctx, &pb.ListRewardsRequest{})
	require.NoError(t, err)
	require.Len(t, res.Rewards, 0)
}

func TestListRewardsDescending(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	res, err := c.ListRewards(ctx, &pb.ListRewardsRequest{Ascending: false})
	require.NoError(t, err)
	require.Len(t, res.Rewards, 3)
	requireRewardsOrder(t, res.Rewards, false)
}

func TestListRewardsAscending(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	res, err := c.ListRewards(ctx, &pb.ListRewardsRequest{Ascending: true})
	require.NoError(t, err)
	require.Len(t, res.Rewards, 3)
	requireRewardsOrder(t, res.Rewards, true)
}

func TestListRewardsPaging(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org3", "user3", analyticspb.Event_EVENT_BILLING_SETUP)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org3", "user3", analyticspb.Event_EVENT_BUCKET_CREATED)
	numPages := 0
	more := true
	var startAtToken *timestamppb.Timestamp
	for more {
		req := &pb.ListRewardsRequest{Limit: 3}
		if startAtToken != nil {
			req.StartAt = startAtToken
		}
		res, err := c.ListRewards(ctx, req)
		require.NoError(t, err)
		numPages++
		if numPages < 3 {
			require.True(t, res.More)
			require.NotNil(t, res.MoreStartAt)
			require.Len(t, res.Rewards, 3)
		}
		if numPages == 3 {
			require.False(t, res.More)
			require.Nil(t, res.MoreStartAt)
			require.Len(t, res.Rewards, 2)
		}
		startAtToken = res.MoreStartAt
		more = res.More
	}
	require.Equal(t, 3, numPages)
}

func TestClaimNoRewarded(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	_, err := c.Claim(ctx, &pb.ClaimRequest{OrgKey: "org1", ClaimedBy: "me", Amount: 1})
	require.Error(t, err)
}

func TestClaim(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireClaim(t, ctx, c, "org1", "me", 1)
}

func TestClaimAllAvailable(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	bal, err := c.Balance(ctx, &pb.BalanceRequest{OrgKey: "org1"})
	require.NoError(t, err)
	require.Greater(t, bal.Available, int32(0))
	requireClaim(t, ctx, c, "org1", "me", bal.Available)
}

func TestClaimTooMuch(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	_, err := c.Claim(ctx, &pb.ClaimRequest{OrgKey: "org1", ClaimedBy: "me", Amount: 100})
	require.Error(t, err)
}

func TestFinalizeClaim(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	claim := requireClaim(t, ctx, c, "org1", "me", 1)
	requireFinalizeClaim(t, ctx, c, claim.Id, "org1", "acid", "")
}

func TestFinalizeNonExistentClaim(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireClaim(t, ctx, c, "org1", "me", 1)
	_, err := c.FinalizeClaim(ctx, &pb.FinalizeClaimRequest{Id: primitive.NewObjectID().Hex(), OrgKey: "org1", TxnCid: "acid"})
	require.Error(t, err)
}

func TestListClaimsEmpty(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireClaim(t, ctx, c, "org1", "user1", 1)
	requireClaim(t, ctx, c, "org1", "user2", 1)
	requireClaim(t, ctx, c, "org1", "user3", 1)
	res, err := c.ListClaims(ctx, &pb.ListClaimsRequest{})
	require.NoError(t, err)
	require.Len(t, res.Claims, 3)
}

func TestListClaimsOrgKey(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_ORG_CREATED)
	requireClaim(t, ctx, c, "org1", "user1", 1)
	requireClaim(t, ctx, c, "org1", "user2", 1)
	requireClaim(t, ctx, c, "org2", "user2", 1)
	res, err := c.ListClaims(ctx, &pb.ListClaimsRequest{OrgKeyFilter: "org1"})
	require.NoError(t, err)
	require.Len(t, res.Claims, 2)
}

func TestListClaimsClaimedBy(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireClaim(t, ctx, c, "org1", "user1", 1)
	requireClaim(t, ctx, c, "org1", "user2", 1)
	requireClaim(t, ctx, c, "org1", "user3", 1)
	res, err := c.ListClaims(ctx, &pb.ListClaimsRequest{ClaimedByFilter: "user2"})
	require.NoError(t, err)
	require.Len(t, res.Claims, 1)
}

func TestListClaimsOrgKeyClaimedBy(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_ORG_CREATED)
	requireClaim(t, ctx, c, "org1", "user1", 1)
	requireClaim(t, ctx, c, "org1", "user2", 1)
	requireClaim(t, ctx, c, "org1", "user2", 1)
	requireClaim(t, ctx, c, "org2", "user2", 1)
	res, err := c.ListClaims(ctx, &pb.ListClaimsRequest{OrgKeyFilter: "org1", ClaimedByFilter: "user2"})
	require.NoError(t, err)
	require.Len(t, res.Claims, 2)
}

func TestListClaimsStateFilterEmpty(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_ORG_CREATED)
	c1 := requireClaim(t, ctx, c, "org1", "user1", 1)
	c2 := requireClaim(t, ctx, c, "org1", "user2", 1)
	requireClaim(t, ctx, c, "org1", "user3", 1)
	requireFinalizeClaim(t, ctx, c, c1.Id, "org1", "acid", "")
	requireFinalizeClaim(t, ctx, c, c2.Id, "org1", "", "it failed")
	res, err := c.ListClaims(ctx, &pb.ListClaimsRequest{})
	require.NoError(t, err)
	require.Len(t, res.Claims, 3)
}

func TestListClaimsStateFilterUnspecified(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_ORG_CREATED)
	c1 := requireClaim(t, ctx, c, "org1", "user1", 1)
	c2 := requireClaim(t, ctx, c, "org1", "user2", 1)
	requireClaim(t, ctx, c, "org1", "user3", 1)
	requireFinalizeClaim(t, ctx, c, c1.Id, "org1", "acid", "")
	requireFinalizeClaim(t, ctx, c, c2.Id, "org1", "", "it failed")
	res, err := c.ListClaims(ctx, &pb.ListClaimsRequest{StateFilter: pb.ClaimState_CLAIM_STATE_UNSPECIFIED})
	require.NoError(t, err)
	require.Len(t, res.Claims, 3)
}

func TestListClaimsStateFilterPending(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_ORG_CREATED)
	c1 := requireClaim(t, ctx, c, "org1", "user1", 1)
	c2 := requireClaim(t, ctx, c, "org1", "user2", 1)
	requireClaim(t, ctx, c, "org1", "user3", 1)
	requireFinalizeClaim(t, ctx, c, c1.Id, "org1", "acid", "")
	requireFinalizeClaim(t, ctx, c, c2.Id, "org1", "", "it failed")
	res, err := c.ListClaims(ctx, &pb.ListClaimsRequest{StateFilter: pb.ClaimState_CLAIM_STATE_PENDING})
	require.NoError(t, err)
	require.Len(t, res.Claims, 1)
	require.Equal(t, "org1", res.Claims[0].OrgKey)
	require.Equal(t, "user3", res.Claims[0].ClaimedBy)
	require.Equal(t, int32(1), res.Claims[0].Amount)
}

func TestListClaimsStateFilterComplete(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_ORG_CREATED)
	c1 := requireClaim(t, ctx, c, "org1", "user1", 1)
	c2 := requireClaim(t, ctx, c, "org1", "user2", 1)
	requireClaim(t, ctx, c, "org1", "user3", 1)
	requireFinalizeClaim(t, ctx, c, c1.Id, "org1", "acid", "")
	requireFinalizeClaim(t, ctx, c, c2.Id, "org1", "", "it failed")
	res, err := c.ListClaims(ctx, &pb.ListClaimsRequest{StateFilter: pb.ClaimState_CLAIM_STATE_COMPLETE})
	require.NoError(t, err)
	require.Len(t, res.Claims, 1)
	require.Equal(t, "org1", res.Claims[0].OrgKey)
	require.Equal(t, "user1", res.Claims[0].ClaimedBy)
	require.Equal(t, int32(1), res.Claims[0].Amount)
}

func TestListClaimsStateFilterFailed(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_ORG_CREATED)
	c1 := requireClaim(t, ctx, c, "org1", "user1", 1)
	c2 := requireClaim(t, ctx, c, "org1", "user2", 1)
	requireClaim(t, ctx, c, "org1", "user3", 1)
	requireFinalizeClaim(t, ctx, c, c1.Id, "org1", "acid", "")
	requireFinalizeClaim(t, ctx, c, c2.Id, "org1", "", "it failed")
	res, err := c.ListClaims(ctx, &pb.ListClaimsRequest{StateFilter: pb.ClaimState_CLAIM_STATE_FAILED})
	require.NoError(t, err)
	require.Len(t, res.Claims, 1)
	require.Equal(t, "org1", res.Claims[0].OrgKey)
	require.Equal(t, "user2", res.Claims[0].ClaimedBy)
	require.Equal(t, int32(1), res.Claims[0].Amount)
}

func TestListClaimsDescending(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireClaim(t, ctx, c, "org1", "user1", 1)
	time.Sleep(time.Millisecond * 500)
	requireClaim(t, ctx, c, "org1", "user1", 1)
	time.Sleep(time.Millisecond * 500)
	requireClaim(t, ctx, c, "org1", "user1", 1)
	res, err := c.ListClaims(ctx, &pb.ListClaimsRequest{Ascending: false})
	require.NoError(t, err)
	require.Len(t, res.Claims, 3)
	requireClaimsOrder(t, res.Claims, false)
}

func TestListClaimsAscending(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireClaim(t, ctx, c, "org1", "user1", 1)
	time.Sleep(time.Millisecond * 500)
	requireClaim(t, ctx, c, "org1", "user1", 1)
	time.Sleep(time.Millisecond * 500)
	requireClaim(t, ctx, c, "org1", "user1", 1)
	res, err := c.ListClaims(ctx, &pb.ListClaimsRequest{Ascending: true})
	require.NoError(t, err)
	require.Len(t, res.Claims, 3)
	requireClaimsOrder(t, res.Claims, true)
}

func TestListClaimsPaging(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_ORG_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_ORG_CREATED)

	requireClaim(t, ctx, c, "org1", "user1", 1)
	time.Sleep(time.Millisecond * 500)
	requireClaim(t, ctx, c, "org1", "user1", 1)
	time.Sleep(time.Millisecond * 500)
	requireClaim(t, ctx, c, "org1", "user1", 1)
	time.Sleep(time.Millisecond * 500)
	requireClaim(t, ctx, c, "org1", "user1", 1)
	time.Sleep(time.Millisecond * 500)
	requireClaim(t, ctx, c, "org2", "user2", 1)
	time.Sleep(time.Millisecond * 500)
	requireClaim(t, ctx, c, "org2", "user2", 1)
	time.Sleep(time.Millisecond * 500)
	requireClaim(t, ctx, c, "org2", "user2", 1)
	time.Sleep(time.Millisecond * 500)
	requireClaim(t, ctx, c, "org2", "user2", 1)
	numPages := 0
	more := true
	var startAtToken *timestamppb.Timestamp
	for more {
		req := &pb.ListClaimsRequest{Limit: 3}
		if startAtToken != nil {
			req.StartAt = startAtToken
		}
		res, err := c.ListClaims(ctx, req)
		require.NoError(t, err)
		numPages++
		if numPages < 3 {
			require.True(t, res.More)
			require.NotNil(t, res.MoreStartAt)
			require.Len(t, res.Claims, 3)
		}
		if numPages == 3 {
			require.False(t, res.More)
			require.Nil(t, res.MoreStartAt)
			require.Len(t, res.Claims, 2)
		}
		startAtToken = res.MoreStartAt
		more = res.More
	}
	require.Equal(t, 3, numPages)
}

func TestEmtptyBalance(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireBalance(t, ctx, c, "org1", 0, 0, 0, 0)
}

func TestRewardedBalance(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireBalance(t, ctx, c, "org1", 2, 0, 0, 2)
}

func TestPendingBalance(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireClaim(t, ctx, c, "org1", "me", 1)
	requireBalance(t, ctx, c, "org1", 2, 1, 0, 1)
}

func TestClaimedBalance(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	claim := requireClaim(t, ctx, c, "org1", "me", 1)
	requireFinalizeClaim(t, ctx, c, claim.Id, "org1", "acid", "")
	requireBalance(t, ctx, c, "org1", 2, 0, 1, 1)
}

func requireSetup(t *testing.T, ctx context.Context) (pb.FilRewardsServiceClient, func()) {
	listener := bufconn.Listen(bufSize)

	conf := Config{
		Listener:          listener,
		MongoUri:          test.GetMongoUri(),
		MongoDbName:       util.MakeToken(12),
		BaseAttoFILReward: 2,
	}
	s, err := New(ctx, conf)
	require.NoError(t, err)

	bufDialer := func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}

	conn, err := grpc.Dial("bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	require.NoError(t, err)
	client := pb.NewFilRewardsServiceClient(conn)

	cleanup := func() {
		conn.Close()
		s.Close()
	}

	return client, cleanup
}

func requireRewardsOrder(t *testing.T, res []*pb.Reward, ascending bool) {
	var last *time.Time
	for _, rec := range res {
		if last != nil {
			a := *last
			b := rec.CreatedAt.AsTime()
			if ascending {
				a = rec.CreatedAt.AsTime()
				b = *last
			}
			require.True(t, a.After(b))
		}
		t := rec.CreatedAt.AsTime()
		last = &t
	}
}

func requireClaimsOrder(t *testing.T, res []*pb.Claim, ascending bool) {
	var last *time.Time
	for _, rec := range res {
		if last != nil {
			a := *last
			b := rec.CreatedAt.AsTime()
			if ascending {
				a = rec.CreatedAt.AsTime()
				b = *last
			}
			require.True(t, a.After(b))
		}
		t := rec.CreatedAt.AsTime()
		last = &t
	}
}

func requireProcessedEvent(t *testing.T, ctx context.Context, c pb.FilRewardsServiceClient, orgKey, devKey string, event analyticspb.Event) *pb.Reward {
	req := &pb.ProcessAnalyticsEventRequest{
		OrgKey:         orgKey,
		DevKey:         devKey,
		AnalyticsEvent: event,
	}
	res, err := c.ProcessAnalyticsEvent(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res.Reward)
	return res.Reward
}

func requireNoProcessedEvent(t *testing.T, ctx context.Context, c pb.FilRewardsServiceClient, orgKey, devKey string, event analyticspb.Event) {
	req := &pb.ProcessAnalyticsEventRequest{
		OrgKey:         orgKey,
		DevKey:         devKey,
		AnalyticsEvent: event,
	}
	res, err := c.ProcessAnalyticsEvent(ctx, req)
	require.NoError(t, err)
	require.Nil(t, res.Reward)
}

func requireClaim(t *testing.T, ctx context.Context, c pb.FilRewardsServiceClient, orgKey, claimedBy string, amount int32) *pb.Claim {
	res, err := c.Claim(ctx, &pb.ClaimRequest{OrgKey: orgKey, ClaimedBy: claimedBy, Amount: amount})
	require.NoError(t, err)
	require.Equal(t, orgKey, res.Claim.OrgKey)
	require.Equal(t, amount, res.Claim.Amount)
	require.Equal(t, claimedBy, res.Claim.ClaimedBy)
	require.Equal(t, pb.ClaimState_CLAIM_STATE_PENDING, res.Claim.State)
	require.Greater(t, len(res.Claim.Id), 0)
	return res.Claim
}

func requireFinalizeClaim(t *testing.T, ctx context.Context, c pb.FilRewardsServiceClient, id, orgKey, txnCid, failureMsg string) {
	_, err := c.FinalizeClaim(ctx, &pb.FinalizeClaimRequest{Id: id, OrgKey: orgKey, TxnCid: txnCid, FailureMessage: failureMsg})
	require.NoError(t, err)
}

func requireBalance(t *testing.T, ctx context.Context, c pb.FilRewardsServiceClient, orgKey string, rewarded, pending, claimed, available int32) *pb.BalanceResponse {
	res, err := c.Balance(ctx, &pb.BalanceRequest{OrgKey: orgKey})
	require.NoError(t, err)
	require.Equal(t, rewarded, res.Rewarded)
	require.Equal(t, pending, res.Pending)
	require.Equal(t, claimed, res.Claimed)
	require.Equal(t, available, res.Available)
	return res
}
