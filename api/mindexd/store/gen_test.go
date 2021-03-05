package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/textileio/textile/v2/api/mindexd/model"
)

func TestMinerIndexGeneration(t *testing.T) {
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

	// Check we have 3 miners.
	all, err := s.GetAllMiners(ctx)
	require.NoError(t, err)
	require.Len(t, all, 2)

	// Check non-existant miner
	_, err = s.GetMinerInfo(ctx, "i-dont-exist")
	require.Equal(t, ErrMinerNotExists, err)

	// Miner f0100
	{
		f0100, err := s.GetMinerInfo(ctx, "f0100")
		require.NoError(t, err)
		require.Equal(t, "f0100", f0100.MinerID)
		require.False(t, f0100.UpdatedAt.IsZero())
		require.False(t, f0100.Textile.UpdatedAt.IsZero())
		require.Len(t, f0100.Textile.Regions, 3)
		require.Equal(t, 4, f0100.Textile.DealsSummary.Total)
		require.Equal(t, int64(303), f0100.Textile.DealsSummary.Last.Unix())
		require.Equal(t, 1, f0100.Textile.DealsSummary.Failures)
		require.Equal(t, int64(304), f0100.Textile.DealsSummary.LastFailure.Unix())
		require.Equal(t, 4, f0100.Textile.RetrievalsSummary.Total)
		require.Equal(t, int64(1003), f0100.Textile.RetrievalsSummary.Last.Unix())
		require.Equal(t, 2, f0100.Textile.RetrievalsSummary.Failures)
		require.Equal(t, int64(1012), f0100.Textile.RetrievalsSummary.LastFailure.Unix())

		// <south-america>
		f0100_south_america := f0100.Textile.Regions["005"]
		// !Deals
		require.Equal(t, 2, f0100_south_america.Deals.Total)                       // Test 2 successes.
		require.Equal(t, int64(301), f0100_south_america.Deals.Last.Unix())        // Max of successes.
		require.Equal(t, 1, f0100_south_america.Deals.Failures)                    // Test single failure.
		require.Equal(t, int64(304), f0100_south_america.Deals.LastFailure.Unix()) // Single failure date.
		// Transfer
		require.Len(t, f0100_south_america.Deals.TailTransfers, 2)
		require.Equal(t, float64(10), f0100_south_america.Deals.TailTransfers[1].MiBPerSec)
		require.Equal(t, int64(10010), f0100_south_america.Deals.TailTransfers[1].TransferedAt.Unix())
		require.Equal(t, float64(25), f0100_south_america.Deals.TailTransfers[0].MiBPerSec)
		require.Equal(t, int64(10002), f0100_south_america.Deals.TailTransfers[0].TransferedAt.Unix())
		// Sealing
		require.Len(t, f0100_south_america.Deals.TailSealed, 2)
		require.Equal(t, 60*60, f0100_south_america.Deals.TailSealed[1].DurationSeconds)
		require.Equal(t, int64(23600), f0100_south_america.Deals.TailSealed[1].SealedAt.Unix())
		require.Equal(t, 600*60, f0100_south_america.Deals.TailSealed[0].DurationSeconds)
		require.Equal(t, int64(56000), f0100_south_america.Deals.TailSealed[0].SealedAt.Unix())

		// !Retrievals
		require.Equal(t, 2, f0100_south_america.Retrievals.Total)
		require.Equal(t, 1, f0100_south_america.Retrievals.Failures)
		require.Equal(t, int64(1002), f0100_south_america.Retrievals.Last.Unix())
		require.Equal(t, int64(1011), f0100_south_america.Retrievals.LastFailure.Unix())
		require.Len(t, f0100_south_america.Retrievals.TailTransfers, 2)
		require.Equal(t, float64(10), f0100_south_america.Retrievals.TailTransfers[0].MiBPerSec)
		require.Equal(t, int64(1010), f0100_south_america.Retrievals.TailTransfers[0].TransferedAt.Unix())
		require.Equal(t, float64(50), f0100_south_america.Retrievals.TailTransfers[1].MiBPerSec)
		require.Equal(t, int64(1020), f0100_south_america.Retrievals.TailTransfers[1].TransferedAt.Unix())

		// </south-america>

		// <north-america>
		f0100_north_america := f0100.Textile.Regions["003"]
		// !Deals
		require.Equal(t, 1, f0100_north_america.Deals.Total)
		require.Equal(t, int64(302), f0100_north_america.Deals.Last.Unix())
		require.Equal(t, 0, f0100_north_america.Deals.Failures)
		require.True(t, f0100_north_america.Deals.LastFailure.IsZero())
		// Transfers
		require.Len(t, f0100_north_america.Deals.TailTransfers, 1)
		require.Equal(t, float64(1), f0100_north_america.Deals.TailTransfers[0].MiBPerSec)
		require.Equal(t, int64(10010), f0100_north_america.Deals.TailTransfers[0].TransferedAt.Unix())
		// Sealing
		require.Len(t, f0100_north_america.Deals.TailSealed, 1)
		require.Equal(t, 300*60, f0100_north_america.Deals.TailSealed[0].DurationSeconds)
		require.Equal(t, int64(38000), f0100_north_america.Deals.TailSealed[0].SealedAt.Unix())

		// !Retrievals
		require.Equal(t, 2, f0100_north_america.Retrievals.Total)
		require.Equal(t, 1, f0100_north_america.Retrievals.Failures)
		require.Equal(t, int64(1003), f0100_north_america.Retrievals.Last.Unix())
		require.Equal(t, int64(1012), f0100_north_america.Retrievals.LastFailure.Unix())
		require.Len(t, f0100_north_america.Retrievals.TailTransfers, 1)
		require.Equal(t, float64(6.25), f0100_north_america.Retrievals.TailTransfers[0].MiBPerSec)
		require.Equal(t, int64(1016), f0100_north_america.Retrievals.TailTransfers[0].TransferedAt.Unix())
		// </north-america>

		// <africa>
		f0100_africa := f0100.Textile.Regions["002"]
		// !Deals
		require.Equal(t, 1, f0100_africa.Deals.Total)
		require.Equal(t, int64(303), f0100_africa.Deals.Last.Unix())
		require.Equal(t, 0, f0100_africa.Deals.Failures)
		require.True(t, f0100_africa.Deals.LastFailure.IsZero())
		// Transfers
		require.Len(t, f0100_africa.Deals.TailTransfers, 0) // Our record didn't have transfer data.
		require.Len(t, f0100_africa.Deals.TailSealed, 1)    // But the record *had* sealing data.
		// Sealing
		require.Equal(t, 150*60, f0100_africa.Deals.TailSealed[0].DurationSeconds)
		require.Equal(t, int64(29000), f0100_africa.Deals.TailSealed[0].SealedAt.Unix())

		// !Retrievals
		require.Len(t, f0100_africa.Retrievals.TailTransfers, 0)

		// <africa>
	}

	// Miner f0101
	{
		f0101, err := s.GetMinerInfo(ctx, "f0101")
		require.NoError(t, err)
		require.Equal(t, "f0101", f0101.MinerID)
		require.False(t, f0101.UpdatedAt.IsZero())
		require.False(t, f0101.Textile.UpdatedAt.IsZero())
		require.Len(t, f0101.Textile.Regions, 3)
		require.Equal(t, 2, f0101.Textile.DealsSummary.Total)
		require.Equal(t, int64(311), f0101.Textile.DealsSummary.Last.Unix())
		require.Equal(t, 2, f0101.Textile.DealsSummary.Failures)
		require.Equal(t, int64(311), f0101.Textile.DealsSummary.Last.Unix())
		require.Equal(t, 1, f0101.Textile.RetrievalsSummary.Total)
		require.Equal(t, int64(1050), f0101.Textile.RetrievalsSummary.Last.Unix())
		require.Equal(t, 1, f0101.Textile.RetrievalsSummary.Failures)
		require.Equal(t, int64(1042), f0101.Textile.RetrievalsSummary.LastFailure.Unix())

		// <south-america>
		f0101_south_america := f0101.Textile.Regions["005"]
		// !Deals
		require.Len(t, f0101_south_america.Deals.TailTransfers, 0)
		require.Len(t, f0101_south_america.Deals.TailSealed, 0)

		// !Retrievals
		require.Equal(t, 1, f0101_south_america.Retrievals.Total)
		require.Equal(t, 0, f0101_south_america.Retrievals.Failures)
		require.Equal(t, int64(1050), f0101_south_america.Retrievals.Last.Unix())
		require.Equal(t, time.Time{}.Unix(), f0101_south_america.Retrievals.LastFailure.Unix())
		require.Len(t, f0101_south_america.Retrievals.TailTransfers, 1)
		require.Equal(t, float64(0.1), f0101_south_america.Retrievals.TailTransfers[0].MiBPerSec)
		require.Equal(t, int64(1010), f0101_south_america.Retrievals.TailTransfers[0].TransferedAt.Unix())
		// </south-america>

		// <north-america>
		f0101_north_america := f0101.Textile.Regions["003"]
		// !Deals
		require.Equal(t, 1, f0101_north_america.Deals.Total)
		require.Equal(t, int64(310), f0101_north_america.Deals.Last.Unix())
		require.Equal(t, 0, f0101_north_america.Deals.Failures)
		require.True(t, f0101_north_america.Deals.LastFailure.IsZero())
		// Transfers
		require.Len(t, f0101_north_america.Deals.TailTransfers, 1)
		require.Equal(t, float64(0.2), f0101_north_america.Deals.TailTransfers[0].MiBPerSec)
		require.Equal(t, int64(10010), f0101_north_america.Deals.TailTransfers[0].TransferedAt.Unix())
		// Sealing
		require.Len(t, f0101_north_america.Deals.TailSealed, 1)
		require.Equal(t, 300*60, f0101_north_america.Deals.TailSealed[0].DurationSeconds)
		require.Equal(t, int64(38000), f0101_north_america.Deals.TailSealed[0].SealedAt.Unix())

		// !Retrievals
		require.Equal(t, 0, f0101_north_america.Retrievals.Total)
		require.Equal(t, 1, f0101_north_america.Retrievals.Failures)
		require.Equal(t, time.Time{}.Unix(), f0101_north_america.Retrievals.Last.Unix())
		require.Equal(t, int64(1042), f0101_north_america.Retrievals.LastFailure.Unix())

		require.Len(t, f0101_north_america.Retrievals.TailTransfers, 0)
		// </north-america>

		// <africa>
		f0101_africa := f0101.Textile.Regions["002"]
		// !Deals
		require.Equal(t, 1, f0101_africa.Deals.Total)
		require.Equal(t, int64(311), f0101_africa.Deals.Last.Unix())
		require.Equal(t, 2, f0101_africa.Deals.Failures)                    // Test 2 failures.
		require.Equal(t, int64(313), f0101_africa.Deals.LastFailure.Unix()) // Max of 2 failures.
		// Transfers
		require.Len(t, f0101_north_america.Deals.TailTransfers, 1)
		require.Equal(t, float64(0.2), f0101_africa.Deals.TailTransfers[0].MiBPerSec)
		require.Equal(t, int64(10010), f0101_africa.Deals.TailTransfers[0].TransferedAt.Unix())
		// Sealing
		require.Len(t, f0101_africa.Deals.TailSealed, 0) // Our record didn't have sealing information.
		// !Retrievals
		require.Len(t, f0101_africa.Retrievals.TailTransfers, 0)
	}

	// Get all testing
	all, err = s.GetAllMiners(ctx)
	require.NoError(t, err)
	require.Len(t, all, 2)
}

var (
	// Deals
	testDealsSouthAmerica = []model.PowStorageDealRecord{
		{
			Pending:           false,
			DataTransferStart: time.Unix(10000, 0),
			DataTransferEnd:   time.Unix(10010, 0),
			TransferSize:      1024 * 1024 * 100, // 100MiB in 10 sec =  10MiB/s
			SealingStart:      time.Unix(20000, 0),
			SealingEnd:        time.Unix(23600, 0), // Sealing duration = 60min (1hr)
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "0_1",
				Miner:       "f0100",
			},
			UpdatedAt: time.Unix(300, 0),
		},
		{
			Pending:           false,
			DataTransferStart: time.Unix(10000, 0),
			DataTransferEnd:   time.Unix(10002, 0),
			TransferSize:      1024 * 1024 * 50, // 50MiB in 2 sec =  25MiB/s
			SealingStart:      time.Unix(20000, 0),
			SealingEnd:        time.Unix(56000, 0), // Sealing duration = 600 min (10hr)
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "0_2",
				Miner:       "f0100",
			},
			UpdatedAt: time.Unix(301, 0),
		},
		{
			Pending: false,
			Failed:  true,
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "0_5",
				Miner:       "f0100",
			},
			UpdatedAt: time.Unix(304, 0),
		},
		{
			Pending: true,
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "1_1",
				Miner:       "f0101",
			},
			UpdatedAt: time.Unix(310, 0),
		},
	}
	testDealsNorthAmerica = []model.PowStorageDealRecord{
		{
			Pending:           false,
			DataTransferStart: time.Unix(10000, 0),
			DataTransferEnd:   time.Unix(10010, 0),
			TransferSize:      1024 * 1024 * 10, // 10MiB in 10 sec =  1MiB/s
			SealingStart:      time.Unix(20000, 0),
			SealingEnd:        time.Unix(38000, 0), // Sealing duration = 300 min (5hr)
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "0_3",
				Miner:       "f0100",
			},
			UpdatedAt: time.Unix(302, 0),
		},
		{
			Pending:           false,
			DataTransferStart: time.Unix(10000, 0),
			DataTransferEnd:   time.Unix(10010, 0),
			TransferSize:      1024 * 1024 * 2, // 2MiB in 10 sec =  0.2MiB/s
			SealingStart:      time.Unix(20000, 0),
			SealingEnd:        time.Unix(38000, 0), // Sealing duration = 300 min (5hr)
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "1_2",
				Miner:       "f0101",
			},
			UpdatedAt: time.Unix(310, 0),
		},
	}
	testDealsAfrica = []model.PowStorageDealRecord{
		{
			Pending:           false,
			DataTransferStart: time.Time{},
			DataTransferEnd:   time.Time{}, // Simulate not having transfer times.
			TransferSize:      1024 * 1024 * 1,
			SealingStart:      time.Unix(20000, 0),
			SealingEnd:        time.Unix(29000, 0), // Sealing duration = 150 min (2.5hr)

			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "0_4",
				Miner:       "f0100",
			},
			UpdatedAt: time.Unix(303, 0),
		},
		{
			Pending:           false,
			DataTransferStart: time.Unix(10000, 0),
			DataTransferEnd:   time.Unix(10010, 0),
			TransferSize:      1024 * 1024 * 2, // 2MiB in 10 sec =  0.2MiB/s
			SealingStart:      time.Time{},
			SealingEnd:        time.Time{}, // Simulate not having sealing times.

			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "1_3",
				Miner:       "f0101",
			},
			UpdatedAt: time.Unix(311, 0),
		},
		{
			Pending: false,
			Failed:  true,
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "1_4",
				Miner:       "f0101",
			},
			UpdatedAt: time.Unix(312, 0),
		},
		{
			Pending: false,
			Failed:  true,
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "1_5",
				Miner:       "f0101",
			},
			UpdatedAt: time.Unix(313, 0),
		},
	}

	// Retrievals
	testRetrievalsSouthAmerica = []model.PowRetrievalRecord{
		{
			ID: "0_1",
			DealInfo: model.PowRetrievalRecordDealInfo{
				Miner: "f0100",
			},
			DataTransferStart: time.Unix(1000, 0),
			DataTransferEnd:   time.Unix(1010, 0),
			BytesReceived:     1024 * 1024 * 100, // Rate = 10MiB/s
			UpdatedAt:         time.Unix(1002, 0),
		},
		{
			ID: "0_2",
			DealInfo: model.PowRetrievalRecordDealInfo{
				Miner: "f0100",
			},
			DataTransferStart: time.Unix(1000, 0),
			DataTransferEnd:   time.Unix(1020, 0),
			BytesReceived:     1024 * 1024 * 1000, // Rate = 50MiB/s
			UpdatedAt:         time.Unix(1001, 0),
		},
		{
			ID: "0_5",
			DealInfo: model.PowRetrievalRecordDealInfo{
				Miner: "f0100",
			},
			Failed:    true,
			UpdatedAt: time.Unix(1011, 0),
		},
		{
			ID: "1_1",
			DealInfo: model.PowRetrievalRecordDealInfo{
				Miner: "f0101",
			},
			DataTransferStart: time.Unix(1000, 0),
			DataTransferEnd:   time.Unix(1010, 0),
			BytesReceived:     1024 * 1024 * 1, // Rate = 0.1MiB/s
			UpdatedAt:         time.Unix(1050, 0),
		},
	}
	testRetrievalsNorthAmerica = []model.PowRetrievalRecord{
		{
			ID: "0_3",
			DealInfo: model.PowRetrievalRecordDealInfo{
				Miner: "f0100",
			},
			DataTransferStart: time.Unix(1000, 0),
			DataTransferEnd:   time.Unix(1016, 0),
			BytesReceived:     1024 * 1024 * 100, // Rate = 6.25MiB/s
			UpdatedAt:         time.Unix(1003, 0),
		},
		{
			ID: "0_4",
			DealInfo: model.PowRetrievalRecordDealInfo{
				Miner: "f0100",
			},
			DataTransferStart: time.Time{},
			DataTransferEnd:   time.Time{}, // Simulate not having data.
			BytesReceived:     1024 * 1024 * 100,
		},
		{
			ID: "0_6",
			DealInfo: model.PowRetrievalRecordDealInfo{
				Miner: "f0100",
			},
			Failed:    true,
			UpdatedAt: time.Unix(1012, 0),
		},
		{
			ID: "1_2",
			DealInfo: model.PowRetrievalRecordDealInfo{
				Miner: "f0101",
			},
			Failed:    true, // Fail for f0101, so no data for this region.
			UpdatedAt: time.Unix(1042, 0),
		},
	}
)
