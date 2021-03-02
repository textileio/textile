package store

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuery(t *testing.T) {
	ctx := context.Background()
	db := setup(t, ctx)

	s, err := New(db)
	require.NoError(t, err)

	// Insert some deal-records.
	err = s.PersistStorageDealRecords(ctx, "duke-1", "005", testDealsSouthAmerica)
	require.NoError(t, err)
	err = s.PersistStorageDealRecords(ctx, "duke-2", "003", testDealsNorthAmerica)
	require.NoError(t, err)
	err = s.PersistStorageDealRecords(ctx, "duke-3", "002", testDealsAfrica)
	require.NoError(t, err)
	// Insert some retrieval records.
	err = s.PersistRetrievalRecords(ctx, "duke-1", "005", testRetrievalsSouthAmerica)
	require.NoError(t, err)
	err = s.PersistRetrievalRecords(ctx, "duke-2", "003", testRetrievalsNorthAmerica)
	require.NoError(t, err)

	// Regenerate indexes.
	err = s.UpdateTextileDealsInfo(ctx)
	require.NoError(t, err)
	err = s.UpdateTextileRetrievalsInfo(ctx)
	require.NoError(t, err)

	t.Run("paging", testPaging(s))
	t.Run("sort", testSort(s))
	t.Run("deals/LastSuccess", testDealLastSuccess(s))
	t.Run("deals/TotalSuccess", testDealTotalSuccess(s))
}

func testPaging(s *Store) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		// Get all results
		res, err := s.QueryIndex(ctx, QueryIndexFilters{}, QueryIndexSort{}, 2, 0)
		require.NoError(t, err)
		require.Len(t, res, 2)

		// Get one result less since offset is 1
		res2, err := s.QueryIndex(ctx, QueryIndexFilters{}, QueryIndexSort{}, 2, 1)
		require.NoError(t, err)
		require.Len(t, res2, 1)

		// Check that offested query is one-shifted from non offested.
		require.Equal(t, res[1].MinerID, res2[0].MinerID)

		// Try smaller limit than all results.
		res, err = s.QueryIndex(ctx, QueryIndexFilters{}, QueryIndexSort{}, 1, 0)
		require.NoError(t, err)
		require.Len(t, res, 1)

		// Try offset bigger than result, which should be empty.
		res, err = s.QueryIndex(ctx, QueryIndexFilters{}, QueryIndexSort{}, 1, 3)
		require.NoError(t, err)
		require.Len(t, res, 0)

		// Don't accept limit = 0.
		res, err = s.QueryIndex(ctx, QueryIndexFilters{}, QueryIndexSort{}, 0, 0)
		require.Error(t, err)
	}
}

func testSort(s *Store) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		res, err := s.QueryIndex(ctx, QueryIndexFilters{}, QueryIndexSort{}, 10, 0)
		require.NoError(t, err)
		require.Len(t, res, 2)

		res2, err := s.QueryIndex(ctx, QueryIndexFilters{}, QueryIndexSort{Ascending: true}, 10, 0)
		require.NoError(t, err)
		require.Len(t, res, 2)

		require.Equal(t, res[0].MinerID, res2[1].MinerID)
		require.Equal(t, res[1].MinerID, res2[0].MinerID)
	}
}

func testDealLastSuccess(s *Store) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		res, err := s.QueryIndex(ctx, QueryIndexFilters{}, QueryIndexSort{Ascending: false, Field: SortFieldTextileDealLastSuccessful}, 2, 0)
		require.NoError(t, err)
		require.True(t, res[0].Textile.DealsSummary.Last.After(res[1].Textile.DealsSummary.Last))

		res, err = s.QueryIndex(ctx, QueryIndexFilters{}, QueryIndexSort{Ascending: true, Field: SortFieldTextileDealLastSuccessful}, 2, 0)
		require.NoError(t, err)
		require.True(t, res[0].Textile.DealsSummary.Last.Before(res[1].Textile.DealsSummary.Last))

	}
}

func testDealTotalSuccess(s *Store) func(t *testing.T) {
	return func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		res, err := s.QueryIndex(ctx, QueryIndexFilters{}, QueryIndexSort{Ascending: false, Field: SortFieldTextileDealTotalSuccessful}, 2, 0)
		require.NoError(t, err)
		require.Greater(t, res[0].Textile.DealsSummary.Total, res[1].Textile.DealsSummary.Total)

		res, err = s.QueryIndex(ctx, QueryIndexFilters{}, QueryIndexSort{Ascending: true, Field: SortFieldTextileDealTotalSuccessful}, 2, 0)
		require.NoError(t, err)
		require.Less(t, res[0].Textile.DealsSummary.Total, res[1].Textile.DealsSummary.Total)
	}
}
