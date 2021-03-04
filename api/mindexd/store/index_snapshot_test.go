package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIndexSnapshot(t *testing.T) {
	ctx := context.Background()
	db := setup(t, ctx)

	s, err := New(db)
	require.NoError(t, err)

	last, err := s.GetLastIndexSnapshotTime(ctx)
	require.NoError(t, err)
	require.Equal(t, time.Time{}, last)

	// Generate some data an create an index.
	err = s.PersistStorageDealRecords(ctx, "duke-1", "005", testDealsSouthAmerica)
	require.NoError(t, err)
	err = s.PersistStorageDealRecords(ctx, "duke-2", "003", testDealsNorthAmerica)
	require.NoError(t, err)
	err = s.PersistStorageDealRecords(ctx, "duke-3", "002", testDealsAfrica)
	require.NoError(t, err)
	err = s.PersistRetrievalRecords(ctx, "duke-1", "005", testRetrievalsSouthAmerica)
	require.NoError(t, err)
	err = s.PersistRetrievalRecords(ctx, "duke-2", "003", testRetrievalsNorthAmerica)
	require.NoError(t, err)
	err = s.UpdateTextileDealsInfo(ctx)
	require.NoError(t, err)
	err = s.UpdateTextileRetrievalsInfo(ctx)
	require.NoError(t, err)

	// Generate a snapshot of the index.
	err = s.GenerateMinerIndexSnapshot(ctx)
	require.NoError(t, err)

	last1, err := s.GetLastIndexSnapshotTime(ctx)
	require.NoError(t, err)
	require.True(t, time.Now().Add(-time.Minute).Before(last1))

	// Generate a snapshot again and check last is newer
	// than previous.
	err = s.GenerateMinerIndexSnapshot(ctx)
	require.NoError(t, err)

	last2, err := s.GetLastIndexSnapshotTime(ctx)
	require.NoError(t, err)
	require.True(t, last1.Before(last2))
}
