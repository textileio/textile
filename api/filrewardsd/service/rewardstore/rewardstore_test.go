package rewardstore

import (
	"context"
	"os"
	"testing"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-ds-mongo/test"
	pb "github.com/textileio/textile/v2/api/filrewardsd/pb"
	"github.com/textileio/textile/v2/util"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ctx = context.Background()
)

func TestMain(m *testing.M) {
	logging.SetAllLoggers(logging.LevelError)

	cleanup := test.StartMongoDB()
	exitVal := m.Run()
	cleanup()
	os.Exit(exitVal)
}

func TestNew(t *testing.T) {
	rs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, rs, "org", "dev", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
}

func TestNewRepeatedOrg(t *testing.T) {
	rs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, rs, "org", "dev", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	_, err := rs.New(ctx, "org", "dev2", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED, 1, 1000)
	require.Error(t, err)
}

func TestNewRepeatedDev(t *testing.T) {
	rs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, rs, "org1", "dev", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	_, err := rs.New(ctx, "org2", "dev", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED, 1, 1000)
	require.Error(t, err)
}

func TestAll(t *testing.T) {
	rs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, rs, "org1", "dev1", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org2", "dev2", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org3", "dev3", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	res, err := rs.All(ctx)
	require.NoError(t, err)
	require.Len(t, res, 3)
}

func TestList(t *testing.T) {
	rs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, rs, "org1", "dev1", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org2", "dev2", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org3", "dev3", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	res, err := rs.List(ctx, &pb.ListRewardsRequest{})
	require.NoError(t, err)
	require.Len(t, res, 3)
}

func TestListOrgKey(t *testing.T) {
	rs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, rs, "org1", "dev1", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org1", "dev1", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_CREATED)
	requireNew(t, rs, "org2", "dev2", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	res, err := rs.List(ctx, &pb.ListRewardsRequest{OrgKeyFilter: "org1"})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListDevKey(t *testing.T) {
	rs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, rs, "org", "dev1", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org", "dev1", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_CREATED)
	requireNew(t, rs, "org", "dev2", pb.RewardType_REWARD_TYPE_FIRST_KEY_ACCOUNT_CREATED)
	res, err := rs.List(ctx, &pb.ListRewardsRequest{DevKeyFilter: "dev1"})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListRewardType(t *testing.T) {
	rs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, rs, "org1", "dev1", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org2", "dev2", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org1", "dev2", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_CREATED)
	res, err := rs.List(ctx, &pb.ListRewardsRequest{RewardTypeFilter: pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListDescending(t *testing.T) {
	rs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, rs, "org1", "dev1", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org2", "dev2", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org3", "dev3", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	res, err := rs.List(ctx, &pb.ListRewardsRequest{Ascending: false})
	require.NoError(t, err)
	require.Len(t, res, 3)
	requireRewardsOrder(t, res, false)
}

func TestListAscending(t *testing.T) {
	rs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, rs, "org1", "dev1", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org2", "dev2", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org3", "dev3", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	res, err := rs.List(ctx, &pb.ListRewardsRequest{Ascending: true})
	require.NoError(t, err)
	require.Len(t, res, 3)
	requireRewardsOrder(t, res, true)
}

func TestListPaging(t *testing.T) {
	rs, cleanup := requireSetup(t)
	defer cleanup()
	requireNew(t, rs, "org1", "dev1", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org2", "dev2", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org3", "dev3", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org4", "dev4", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org5", "dev5", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org6", "dev6", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org7", "dev7", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	requireNew(t, rs, "org8", "dev8", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)

	pageResults := func(ascending bool) {
		page := int64(0)
		for {
			conf := &pb.ListRewardsRequest{
				PageSize:  3,
				Page:      page,
				Ascending: ascending,
			}
			res, err := rs.List(ctx, conf)
			require.NoError(t, err)
			if len(res) == 0 {
				break
			}
			requireRewardsOrder(t, res, ascending)
			if page < 2 {
				require.Len(t, res, 3)
			}
			if page == 2 {
				require.Len(t, res, 2)
			}
			page++
		}
		require.Equal(t, int64(3), page)
	}
	pageResults(false)
	pageResults(true)
}

func TestTotalNanoFilRewards(t *testing.T) {
	rs, cleanup := requireSetup(t)
	defer cleanup()
	r1 := requireNew(t, rs, "org1", "dev1", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED)
	r2 := requireNew(t, rs, "org1", "dev1", pb.RewardType_REWARD_TYPE_FIRST_BUCKET_CREATED)
	r3 := requireNew(t, rs, "org1", "dev1", pb.RewardType_REWARD_TYPE_FIRST_KEY_ACCOUNT_CREATED)
	res, err := rs.TotalNanoFilRewarded(ctx, "org1")
	require.NoError(t, err)
	expTotal := r1.BaseNanoFilReward*r1.Factor + r2.BaseNanoFilReward*r2.Factor + r3.BaseNanoFilReward*r3.Factor
	require.Equal(t, expTotal, res)
}

func requireNew(t *testing.T, rs *RewardStore, orgKey, devKey string, rewardType pb.RewardType) *pb.Reward {
	reward, err := rs.New(ctx, orgKey, devKey, rewardType, 1, 1000)
	require.NoError(t, err)
	require.Equal(t, orgKey, reward.OrgKey)
	require.Equal(t, devKey, reward.DevKey)
	require.Equal(t, rewardType, reward.Type)
	require.Equal(t, int64(1), reward.Factor)
	require.Equal(t, int64(1000), reward.BaseNanoFilReward)
	return reward
}

func requireRewardsOrder(t *testing.T, rewards []*pb.Reward, ascending bool) {
	var last *time.Time
	for _, reward := range rewards {
		if last != nil {
			a := *last
			b := reward.CreatedAt.AsTime()
			if ascending {
				a = reward.CreatedAt.AsTime()
				b = *last
			}
			require.True(t, a.After(b))
		}
		t := reward.CreatedAt.AsTime()
		last = &t
	}
}

func requireSetup(t *testing.T) (*RewardStore, func()) {
	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(test.GetMongoUri()))
	require.NoError(t, err)
	db := mongoClient.Database(util.MakeToken(12))
	rs, err := New(db, true)
	require.NoError(t, err)

	cleanup := func() {
		require.NoError(t, mongoClient.Disconnect(ctx))
	}

	return rs, cleanup
}
