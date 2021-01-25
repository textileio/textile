package filrewards

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-ds-mongo/test"
	"github.com/textileio/textile/v2/api/billingd/analytics"
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
	f := requireFilRewards(t)
	rec := requireProcessedEvent(t, f, "user1", analytics.BillingSetup)
	require.Equal(t, "user1", rec.Key)
	require.Equal(t, InitialBillingSetup, rec.Event)
}

func TestProcessDuplicateEvent(t *testing.T) {
	f := requireFilRewards(t)
	requireProcessedEvent(t, f, "user1", analytics.BillingSetup)
	requireNoProcessedEvent(t, f, "user1", analytics.BillingSetup)
}

func TestDuplicateFromInitializedCache(t *testing.T) {
	f1, err := New(ctx, Config{DBURI: test.GetMongoUri(), DBName: "mydb", CollectionName: "filrewards"})
	require.NoError(t, err)
	requireProcessedEvent(t, f1, "user1", analytics.BillingSetup)
	requireProcessedEvent(t, f1, "user1", analytics.BucketCreated)
	requireProcessedEvent(t, f1, "user2", analytics.BillingSetup)
	f2, err := New(ctx, Config{DBURI: test.GetMongoUri(), DBName: "mydb", CollectionName: "filrewards"})
	require.NoError(t, err)
	requireNoProcessedEvent(t, f2, "user1", analytics.BillingSetup)
	requireNoProcessedEvent(t, f2, "user1", analytics.BucketCreated)
	requireNoProcessedEvent(t, f2, "user2", analytics.BillingSetup)
}

func TestGet(t *testing.T) {
	f := requireFilRewards(t)
	requireProcessedEvent(t, f, "user1", analytics.BillingSetup)
	requireProcessedEvent(t, f, "user2", analytics.BillingSetup)
	rec, err := f.GetRewardRecord(ctx, "user1", InitialBillingSetup)
	require.NoError(t, err)
	require.Equal(t, "user1", rec.Key)
	require.Equal(t, InitialBillingSetup, rec.Event)
}

func TestGetWrongEvent(t *testing.T) {
	f := requireFilRewards(t)
	requireProcessedEvent(t, f, "user1", analytics.BillingSetup)
	rec, err := f.GetRewardRecord(ctx, "user1", FirstBucketCreated)
	require.Equal(t, ErrRecordNotFound, err)
	require.Nil(t, rec)
}

func TestGetWrongKey(t *testing.T) {
	f := requireFilRewards(t)
	requireProcessedEvent(t, f, "user1", analytics.BillingSetup)
	rec, err := f.GetRewardRecord(ctx, "user2", InitialBillingSetup)
	require.Equal(t, ErrRecordNotFound, err)
	require.Nil(t, rec)
}

func TestList(t *testing.T) {
	f := requireFilRewards(t)
	requireProcessedEvent(t, f, "user1", analytics.BillingSetup)
	requireProcessedEvent(t, f, "user1", analytics.BucketCreated)
	requireProcessedEvent(t, f, "user1", analytics.BucketArchiveCreated)
	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{})
	require.NoError(t, err)
	require.Len(t, res, 3)
}

func TestListKeyFilter(t *testing.T) {
	f := requireFilRewards(t)
	requireProcessedEvent(t, f, "user1", analytics.BillingSetup)
	requireProcessedEvent(t, f, "user1", analytics.BucketCreated)
	requireProcessedEvent(t, f, "user2", analytics.BucketArchiveCreated)
	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{KeyFilter: "user1"})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListEventFilter(t *testing.T) {
	f := requireFilRewards(t)
	requireProcessedEvent(t, f, "user1", analytics.BillingSetup)
	requireProcessedEvent(t, f, "user1", analytics.BucketCreated)
	requireProcessedEvent(t, f, "user1", analytics.BucketArchiveCreated)
	requireProcessedEvent(t, f, "user2", analytics.BillingSetup)
	requireProcessedEvent(t, f, "user2", analytics.BucketCreated)
	requireProcessedEvent(t, f, "user2", analytics.BucketArchiveCreated)
	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{EventFilter: InitialBillingSetup})
	require.NoError(t, err)
	require.Len(t, res, 2)
}

func TestListKeyAndEventFilters(t *testing.T) {
	f := requireFilRewards(t)
	requireProcessedEvent(t, f, "user1", analytics.BillingSetup)
	requireProcessedEvent(t, f, "user1", analytics.BucketCreated)
	requireProcessedEvent(t, f, "user1", analytics.BucketArchiveCreated)
	requireProcessedEvent(t, f, "user2", analytics.BillingSetup)
	requireProcessedEvent(t, f, "user2", analytics.BucketCreated)
	requireProcessedEvent(t, f, "user2", analytics.BucketArchiveCreated)
	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{KeyFilter: "user1", EventFilter: InitialBillingSetup})
	require.NoError(t, err)
	require.Len(t, res, 1)
}

func TestListDescending(t *testing.T) {
	f := requireFilRewards(t)
	requireProcessedEvent(t, f, "user1", analytics.BillingSetup)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, f, "user1", analytics.BucketCreated)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, f, "user1", analytics.BucketArchiveCreated)
	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{Ascending: false})
	require.NoError(t, err)
	require.Len(t, res, 3)
	requireOrder(t, res, false)
}

func TestListAscending(t *testing.T) {
	f := requireFilRewards(t)
	requireProcessedEvent(t, f, "user1", analytics.BillingSetup)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, f, "user1", analytics.BucketCreated)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, f, "user1", analytics.BucketArchiveCreated)
	res, _, _, err := f.ListRewardRecords(ctx, ListRewardRecordsOptions{Ascending: true})
	require.NoError(t, err)
	require.Len(t, res, 3)
	requireOrder(t, res, true)
}

func TestListPaging(t *testing.T) {
	f := requireFilRewards(t)
	requireProcessedEvent(t, f, "user1", analytics.BillingSetup)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, f, "user1", analytics.BucketCreated)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, f, "user1", analytics.BucketArchiveCreated)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, f, "user2", analytics.BillingSetup)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, f, "user2", analytics.BucketCreated)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, f, "user2", analytics.BucketArchiveCreated)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, f, "user3", analytics.BillingSetup)
	time.Sleep(time.Millisecond * 500)
	requireProcessedEvent(t, f, "user3", analytics.BucketCreated)
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

func requireFilRewards(t *testing.T) *FilRewards {
	f, err := New(ctx, Config{DBURI: test.GetMongoUri(), DBName: util.MakeToken(12), CollectionName: "filrewards"})
	require.NoError(t, err)
	return f
}

func requireProcessedEvent(t *testing.T, f *FilRewards, key string, event analytics.Event) *RewardRecord {
	r, err := f.ProcessEvent(ctx, key, mongodb.Dev, event)
	require.NoError(t, err)
	require.NotNil(t, r)
	return r
}

func requireNoProcessedEvent(t *testing.T, f *FilRewards, key string, event analytics.Event) {
	r, err := f.ProcessEvent(ctx, key, mongodb.Dev, event)
	require.NoError(t, err)
	require.Nil(t, r)
}
