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

func TestProcessDuplicateEvent(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireNoProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
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
	requireProcessedEvent(t, ctx, c1, "org1", "user2", analyticspb.Event_EVENT_BILLING_SETUP)

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
	requireNoProcessedEvent(t, ctx, c2, "org1", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
}

// func TestClaimReward(t *testing.T) {
// 	t.Parallel()
// 	c, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	requireProcessedEvent(t, ctx, c, "user1", analyticspb.Event_EVENT_BILLING_SETUP)
// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_INITIAL_BILLING_SETUP)
// 	rec := requireGetRewardRecord(t, ctx, c, "user1", pb.Reward_REWARD_INITIAL_BILLING_SETUP)
// 	require.NotNil(t, rec.ClaimedAt)
// }

// func TestClaimRewardTwice(t *testing.T) {
// 	t.Parallel()
// 	c, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	requireProcessedEvent(t, ctx, c, "user1", analyticspb.Event_EVENT_BILLING_SETUP)
// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_INITIAL_BILLING_SETUP)
// 	req := &pb.ClaimRequest{
// 		Key:    "user1",
// 		Reward: pb.Reward_REWARD_INITIAL_BILLING_SETUP,
// 	}
// 	_, err := c.Claim(ctx, req)
// 	st, ok := status.FromError(err)
// 	require.True(t, ok)
// 	require.Equal(t, codes.AlreadyExists, st.Code())
// }

// func TestClaimNonExistantReward(t *testing.T) {
// 	t.Parallel()
// 	c, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	req := &pb.ClaimRequest{
// 		Key:    "user1",
// 		Reward: pb.Reward_REWARD_INITIAL_BILLING_SETUP,
// 	}
// 	_, err := c.Claim(ctx, req)
// 	st, ok := status.FromError(err)
// 	require.True(t, ok)
// 	require.Equal(t, codes.NotFound, st.Code())
// }

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
	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	res, err := c.ListRewards(ctx, &pb.ListRewardsRequest{OrgKeyFilter: "org1"})
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
	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
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
	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	res, err := c.ListRewards(ctx, &pb.ListRewardsRequest{OrgKeyFilter: "org1", RewardTypeFilter: pb.RewardType_REWARD_TYPE_INITIAL_BILLING_SETUP})
	require.NoError(t, err)
	require.Len(t, res.Rewards, 1)
}

// func TestListClaimedFilterEmpty(t *testing.T) {
// 	t.Parallel()
// 	c, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
// 	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
// 	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
// 	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
// 	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
// 	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)

// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_INITIAL_BILLING_SETUP)
// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_FIRST_BUCKET_CREATED)
// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_FIRST_BUCKET_ARCHIVE_CREATED)
// 	requireClaimedReward(t, ctx, c, "user2", pb.Reward_REWARD_INITIAL_BILLING_SETUP)

// 	res, err := c.List(ctx, &pb.ListRequest{})
// 	require.NoError(t, err)
// 	require.Len(t, res.RewardRecords, 6)
// }

// func TestListClaimedFilterUnspecified(t *testing.T) {
// 	t.Parallel()
// 	c, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	requireProcessedEvent(t, ctx, c, "user1", analyticspb.Event_EVENT_BILLING_SETUP)
// 	requireProcessedEvent(t, ctx, c, "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
// 	requireProcessedEvent(t, ctx, c, "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
// 	requireProcessedEvent(t, ctx, c, "user2", analyticspb.Event_EVENT_BILLING_SETUP)
// 	requireProcessedEvent(t, ctx, c, "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
// 	requireProcessedEvent(t, ctx, c, "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)

// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_INITIAL_BILLING_SETUP)
// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_FIRST_BUCKET_CREATED)
// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_FIRST_BUCKET_ARCHIVE_CREATED)
// 	requireClaimedReward(t, ctx, c, "user2", pb.Reward_REWARD_INITIAL_BILLING_SETUP)

// 	res, err := c.List(ctx, &pb.ListRequest{ClaimedFilter: pb.ClaimedFilter_CLAIMED_FILTER_UNSPECIFIED})
// 	require.NoError(t, err)
// 	require.Len(t, res.RewardRecords, 6)
// }

// func TestListClaimedFilterClaimed(t *testing.T) {
// 	t.Parallel()
// 	c, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	requireProcessedEvent(t, ctx, c, "user1", analyticspb.Event_EVENT_BILLING_SETUP)
// 	requireProcessedEvent(t, ctx, c, "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
// 	requireProcessedEvent(t, ctx, c, "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
// 	requireProcessedEvent(t, ctx, c, "user2", analyticspb.Event_EVENT_BILLING_SETUP)
// 	requireProcessedEvent(t, ctx, c, "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
// 	requireProcessedEvent(t, ctx, c, "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)

// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_INITIAL_BILLING_SETUP)
// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_FIRST_BUCKET_CREATED)
// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_FIRST_BUCKET_ARCHIVE_CREATED)
// 	requireClaimedReward(t, ctx, c, "user2", pb.Reward_REWARD_INITIAL_BILLING_SETUP)

// 	res, err := c.List(ctx, &pb.ListRequest{ClaimedFilter: pb.ClaimedFilter_CLAIMED_FILTER_CLAIMED})
// 	require.NoError(t, err)
// 	require.Len(t, res.RewardRecords, 4)
// }

// func TestListClaimedFilterUnclaimed(t *testing.T) {
// 	t.Parallel()
// 	c, cleanup := requireSetup(t, ctx)
// 	defer cleanup()
// 	requireProcessedEvent(t, ctx, c, "user1", analyticspb.Event_EVENT_BILLING_SETUP)
// 	requireProcessedEvent(t, ctx, c, "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
// 	requireProcessedEvent(t, ctx, c, "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
// 	requireProcessedEvent(t, ctx, c, "user2", analyticspb.Event_EVENT_BILLING_SETUP)
// 	requireProcessedEvent(t, ctx, c, "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
// 	requireProcessedEvent(t, ctx, c, "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)

// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_INITIAL_BILLING_SETUP)
// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_FIRST_BUCKET_CREATED)
// 	requireClaimedReward(t, ctx, c, "user1", pb.Reward_REWARD_FIRST_BUCKET_ARCHIVE_CREATED)
// 	requireClaimedReward(t, ctx, c, "user2", pb.Reward_REWARD_INITIAL_BILLING_SETUP)

// 	res, err := c.List(ctx, &pb.ListRequest{ClaimedFilter: pb.ClaimedFilter_CLAIMED_FILTER_UNCLAIMED})
// 	require.NoError(t, err)
// 	require.Len(t, res.RewardRecords, 2)
// }

func TestListDescending(t *testing.T) {
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
	requireOrder(t, res.Rewards, false)
}

func TestListAscending(t *testing.T) {
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
	requireOrder(t, res.Rewards, true)
}

func TestListPaging(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org1", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org1", "user3", analyticspb.Event_EVENT_BILLING_SETUP)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, c, "org1", "user3", analyticspb.Event_EVENT_BUCKET_CREATED)
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

func TestBalance(t *testing.T) {
	t.Parallel()
	c, cleanup := requireSetup(t, ctx)
	defer cleanup()
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org1", "user1", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BILLING_SETUP)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_CREATED)
	requireProcessedEvent(t, ctx, c, "org2", "user2", analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED)
	res, err := c.Balance(ctx, &pb.BalanceRequest{OrgKey: "org1"})
	require.NoError(t, err)
	require.Equal(t, res.Rewarded, int32(10))
	require.Equal(t, res.Claimed, int32(0))
	require.Equal(t, res.Available, int32(10))
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

func requireOrder(t *testing.T, res []*pb.Reward, ascending bool) {
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

// func requireClaimedReward(t *testing.T, ctx context.Context, c pb.FilRewardsServiceClient, key string, reward pb.Reward) {
// 	req := &pb.ClaimRequest{
// 		Key:    key,
// 		Reward: reward,
// 	}
// 	_, err := c.Claim(ctx, req)
// 	require.NoError(t, err)
// }
