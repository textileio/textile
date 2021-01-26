package filrewards

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-ds-mongo/test"
	"github.com/textileio/textile/v2/api/billingd/analytics/events"
	"github.com/textileio/textile/v2/mongodb"
	"github.com/textileio/textile/v2/util"
)

var ctx, _ = context.WithTimeout(context.Background(), 1*time.Minute)

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
	f := requireFilRewards(t, ctx)
	rec := requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	require.Equal(t, "user1", rec.Key)
	require.Equal(t, InitialBillingSetup, rec.Reward)
}

func TestProcessDuplicateEvent(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	requireNoProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
}

func TestDuplicateFromInitializedCache(t *testing.T) {
	t.Parallel()
	f1, err := New(ctx, Config{DBURI: test.GetMongoUri(), DBName: "mydb", CollectionName: "filrewards"})
	require.NoError(t, err)
	requireProcessedEvent(t, ctx, f1, "user1", events.BillingSetup)
	requireProcessedEvent(t, ctx, f1, "user1", events.BucketCreated)
	requireProcessedEvent(t, ctx, f1, "user2", events.BillingSetup)
	f2, err := New(ctx, Config{DBURI: test.GetMongoUri(), DBName: "mydb", CollectionName: "filrewards"})
	require.NoError(t, err)
	requireNoProcessedEvent(t, ctx, f2, "user1", events.BillingSetup)
	requireNoProcessedEvent(t, ctx, f2, "user1", events.BucketCreated)
	requireNoProcessedEvent(t, ctx, f2, "user2", events.BillingSetup)
}

func TestClaimReward(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	requireClaimedReward(t, ctx, f, "user1", InitialBillingSetup)
	rec := requireGetRewardRecord(t, ctx, f, "user1", InitialBillingSetup)
	require.NotNil(t, rec.ClaimedAt)
}

func TestClaimRewardTwice(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	requireClaimedReward(t, ctx, f, "user1", InitialBillingSetup)
	err := f.ClaimReward(ctx, "user1", InitialBillingSetup)
	require.Equal(t, ErrRewardAlreadyClaimed, err)
}

func TestClaimNonExistantReward(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	err := f.ClaimReward(ctx, "user1", InitialBillingSetup)
	require.Equal(t, ErrRecordNotFound, err)
}

func TestGet(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user2", events.BillingSetup)
	requireGetRewardRecord(t, ctx, f, "user1", InitialBillingSetup)
}

func TestGetWrongEvent(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	rec, err := f.GetRewardRecord(ctx, "user1", FirstBucketCreated)
	require.Equal(t, ErrRecordNotFound, err)
	require.Nil(t, rec)
}

func TestGetWrongKey(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	rec, err := f.GetRewardRecord(ctx, "user2", InitialBillingSetup)
	require.Equal(t, ErrRecordNotFound, err)
	require.Nil(t, rec)
}

func TestList(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketArchiveCreated)
	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{})
	require.NoError(t, err)
	require.Len(t, res, 3)
}

func TestListKeyFilter(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketArchiveCreated)
	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{KeyFilter: "user1"})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListEventFilter(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketArchiveCreated)
	requireProcessedEvent(t, ctx, f, "user2", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketArchiveCreated)
	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{EventFilter: InitialBillingSetup})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListKeyAndEventFilters(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketArchiveCreated)
	requireProcessedEvent(t, ctx, f, "user2", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketArchiveCreated)
	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{KeyFilter: "user1", EventFilter: InitialBillingSetup})
	require.NoError(t, err)
	require.Len(t, res, 1)
}

func TestListClaimedFilterEmpty(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketArchiveCreated)
	requireProcessedEvent(t, ctx, f, "user2", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketArchiveCreated)

	requireClaimedReward(t, ctx, f, "user1", InitialBillingSetup)
	requireClaimedReward(t, ctx, f, "user1", FirstBucketCreated)
	requireClaimedReward(t, ctx, f, "user1", FirstBucketArchiveCreated)
	requireClaimedReward(t, ctx, f, "user2", InitialBillingSetup)

	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{})
	require.NoError(t, err)
	require.Len(t, res, 6)
}

func TestListClaimedFilterAll(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketArchiveCreated)
	requireProcessedEvent(t, ctx, f, "user2", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketArchiveCreated)

	requireClaimedReward(t, ctx, f, "user1", InitialBillingSetup)
	requireClaimedReward(t, ctx, f, "user1", FirstBucketCreated)
	requireClaimedReward(t, ctx, f, "user1", FirstBucketArchiveCreated)
	requireClaimedReward(t, ctx, f, "user2", InitialBillingSetup)

	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{ClaimedFilter: All})
	require.NoError(t, err)
	require.Len(t, res, 6)
}

func TestListClaimedFilterClaimed(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketArchiveCreated)
	requireProcessedEvent(t, ctx, f, "user2", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketArchiveCreated)

	requireClaimedReward(t, ctx, f, "user1", InitialBillingSetup)
	requireClaimedReward(t, ctx, f, "user1", FirstBucketCreated)
	requireClaimedReward(t, ctx, f, "user1", FirstBucketArchiveCreated)
	requireClaimedReward(t, ctx, f, "user2", InitialBillingSetup)

	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{ClaimedFilter: Claimed})
	require.NoError(t, err)
	require.Len(t, res, 4)
}

func TestListClaimedFilterUnclaimed(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketArchiveCreated)
	requireProcessedEvent(t, ctx, f, "user2", events.BillingSetup)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketCreated)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketArchiveCreated)

	requireClaimedReward(t, ctx, f, "user1", InitialBillingSetup)
	requireClaimedReward(t, ctx, f, "user1", FirstBucketCreated)
	requireClaimedReward(t, ctx, f, "user1", FirstBucketArchiveCreated)
	requireClaimedReward(t, ctx, f, "user2", InitialBillingSetup)

	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{ClaimedFilter: Unclaimed})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListDescending(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketCreated)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketArchiveCreated)
	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{Ascending: false})
	require.NoError(t, err)
	require.Len(t, res, 3)
	requireOrder(t, res, false)
}

func TestListAscending(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketCreated)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketArchiveCreated)
	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{Ascending: true})
	require.NoError(t, err)
	require.Len(t, res, 3)
	requireOrder(t, res, true)
}

func TestListPaging(t *testing.T) {
	t.Parallel()
	f := requireFilRewards(t, ctx)
	requireProcessedEvent(t, ctx, f, "user1", events.BillingSetup)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketCreated)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, f, "user1", events.BucketArchiveCreated)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, f, "user2", events.BillingSetup)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketCreated)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, f, "user2", events.BucketArchiveCreated)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, f, "user3", events.BillingSetup)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, ctx, f, "user3", events.BucketCreated)
	numPages := 0
	more := true
	var startAtToken *time.Time
	for more {
		o := ListRewardRecordsOptions{Limit: 3}
		if startAtToken != nil {
			o.StartAt = startAtToken
		}
		res, m, n, err := f.ListRewardRecords(ctx, o)
		require.NoError(t, err)
		numPages++
		if numPages < 3 {
			require.True(t, m)
			require.NotNil(t, n)
			require.Len(t, res, 3)
		}
		if numPages == 3 {
			require.False(t, m)
			require.Nil(t, n)
			require.Len(t, res, 2)
		}
		startAtToken = n
		more = m
	}
	require.Equal(t, 3, numPages)
}

func requireOrder(t *testing.T, res []RewardRecord, ascending bool) {
	var last *RewardRecord
	for _, rec := range res {
		if last != nil {
			a := last.CreatedAt
			b := rec.CreatedAt
			if ascending {
				a = rec.CreatedAt
				b = last.CreatedAt
			}
			require.True(t, a.After(b))
		}
		t1 := &rec
		t2 := *t1
		last = &t2
	}
}

func requireFilRewards(t *testing.T, ctx context.Context) *FilRewards {
	f, err := New(ctx, Config{DBURI: test.GetMongoUri(), DBName: util.MakeToken(12), CollectionName: "filrewards"})
	require.NoError(t, err)
	return f
}

func requireProcessedEvent(t *testing.T, ctx context.Context, f *FilRewards, key string, event events.Event) *RewardRecord {
	r, err := f.ProcessEvent(ctx, key, mongodb.Dev, event)
	require.NoError(t, err)
	require.NotNil(t, r)
	return r
}

func requireNoProcessedEvent(t *testing.T, ctx context.Context, f *FilRewards, key string, event events.Event) {
	r, err := f.ProcessEvent(ctx, key, mongodb.Dev, event)
	require.NoError(t, err)
	require.Nil(t, r)
}

func requireClaimedReward(t *testing.T, ctx context.Context, f *FilRewards, key string, reward Reward) {
	err := f.ClaimReward(ctx, key, reward)
	require.NoError(t, err)
}

func requireGetRewardRecord(t *testing.T, ctx context.Context, f *FilRewards, key string, reward Reward) *RewardRecord {
	rec, err := f.GetRewardRecord(ctx, key, reward)
	require.NoError(t, err)
	require.Equal(t, key, rec.Key)
	require.Equal(t, reward, rec.Reward)
	return rec
}
