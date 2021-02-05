package recordstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/textile/v2/api/mindexd/model"
)

func TestMinerIndexGeneration(t *testing.T) {
	ctx := context.Background()
	db := setup(t, ctx)

	s, err := New(db)
	require.NoError(t, err)

	// Insert some deal-records.
	err = s.PersistStorageDealRecords(ctx, "duke-1", "south_america", getRegionalGeneralInfoDuke1SouthAmerica)
	require.NoError(t, err)
	err = s.PersistStorageDealRecords(ctx, "duke-2", "north_america", getRegionalGeneralInfoDuke2NorthAmerica)
	require.NoError(t, err)
	err = s.PersistStorageDealRecords(ctx, "duke-3", "africa", getRegionalGeneralInfoDuke3Africa)
	require.NoError(t, err)
	// TTODO: insert some retrieval-records

	// Regenerate indexes.
	err = s.UpdateTextileDealsInfo(ctx)
	require.NoError(t, err)
	//TTODO: UpdateTextileRetrievalsInfo(ctx)

	// Check we have 3 miners.
	count, err := s.SummaryCount(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count)

	// Check non-existant miner
	_, err = s.GetMinerInfo(ctx, "i-dont-exist")
	require.Equal(t, ErrMinerNotExists, err)

	// f0100
	f0100, err := s.GetMinerInfo(ctx, "f0100")
	require.Equal(t, "f0100", f0100.MinerID)
	require.False(t, f0100.UpdatedAt.IsZero())
	require.False(t, f0100.Textile.UpdatedAt.IsZero())
	require.Len(t, f0100.Textile.Regions, 3)
	// TTODO: delete
	//buf, _ := json.MarshalIndent(f0100, "", " ")
	//fmt.Printf("H: %s\n", buf)
	// South-america.
	f0100_south_america := f0100.Textile.Regions["south_america"]
	require.Equal(t, 2, f0100_south_america.Deals.Total)                       // Test 2 successes.
	require.Equal(t, int64(301), f0100_south_america.Deals.Last.Unix())        // Max of successes.
	require.Equal(t, 1, f0100_south_america.Deals.Failures)                    // Test single failure.
	require.Equal(t, int64(304), f0100_south_america.Deals.LastFailure.Unix()) // Single failure date.
	require.Len(t, f0100_south_america.Deals.TailTransfers, 2)
	require.Equal(t, float64(10), f0100_south_america.Deals.TailTransfers[1].MiBPerSec)
	require.Equal(t, int64(10010), f0100_south_america.Deals.TailTransfers[1].TransferedAt.Unix())
	require.Equal(t, float64(25), f0100_south_america.Deals.TailTransfers[0].MiBPerSec)
	require.Equal(t, int64(10002), f0100_south_america.Deals.TailTransfers[0].TransferedAt.Unix())
	require.Len(t, f0100_south_america.Deals.TailSealed, 2)
	require.Equal(t, 60*60, f0100_south_america.Deals.TailSealed[1].DurationSeconds)
	require.Equal(t, int64(23600), f0100_south_america.Deals.TailSealed[1].SealedAt.Unix())
	require.Equal(t, 600*60, f0100_south_america.Deals.TailSealed[0].DurationSeconds)
	require.Equal(t, int64(56000), f0100_south_america.Deals.TailSealed[0].SealedAt.Unix())

	// North-america.
	f0100_north_america := f0100.Textile.Regions["north_america"]
	require.Equal(t, 1, f0100_north_america.Deals.Total)
	require.Equal(t, int64(302), f0100_north_america.Deals.Last.Unix())
	require.Equal(t, 0, f0100_north_america.Deals.Failures)
	require.True(t, f0100_north_america.Deals.LastFailure.IsZero())
	require.Len(t, f0100_north_america.Deals.TailTransfers, 1)
	require.Equal(t, float64(1), f0100_north_america.Deals.TailTransfers[0].MiBPerSec)
	require.Equal(t, int64(10010), f0100_north_america.Deals.TailTransfers[0].TransferedAt.Unix())
	require.Len(t, f0100_north_america.Deals.TailSealed, 1)
	require.Equal(t, 300*60, f0100_north_america.Deals.TailSealed[0].DurationSeconds)
	require.Equal(t, int64(38000), f0100_north_america.Deals.TailSealed[0].SealedAt.Unix())

	// Africa
	f0100_africa := f0100.Textile.Regions["africa"]
	require.Equal(t, 1, f0100_africa.Deals.Total)
	require.Equal(t, int64(303), f0100_africa.Deals.Last.Unix())
	require.Equal(t, 0, f0100_africa.Deals.Failures)
	require.True(t, f0100_africa.Deals.LastFailure.IsZero())
	require.Len(t, f0100_africa.Deals.TailTransfers, 0) // Our record didn't have transfer data.
	require.Len(t, f0100_africa.Deals.TailSealed, 1)    // But the record *had* sealing data.
	require.Equal(t, 150*60, f0100_africa.Deals.TailSealed[0].DurationSeconds)
	require.Equal(t, int64(29000), f0100_africa.Deals.TailSealed[0].SealedAt.Unix())

	// f0101
	f0101, err := s.GetMinerInfo(ctx, "f0101")
	require.Equal(t, "f0101", f0101.MinerID)
	require.False(t, f0101.UpdatedAt.IsZero())
	require.False(t, f0101.Textile.UpdatedAt.IsZero())
	require.Len(t, f0101.Textile.Regions, 2) // Test doesn't include all regions (south-america deal is pending).
	// North-america.
	f0101_north_america := f0101.Textile.Regions["north_america"]
	require.Equal(t, 1, f0101_north_america.Deals.Total)
	require.Equal(t, int64(310), f0101_north_america.Deals.Last.Unix())
	require.Equal(t, 0, f0101_north_america.Deals.Failures)
	require.True(t, f0101_north_america.Deals.LastFailure.IsZero())
	require.Len(t, f0101_north_america.Deals.TailTransfers, 1)
	require.Equal(t, float64(0.2), f0101_north_america.Deals.TailTransfers[0].MiBPerSec)
	require.Equal(t, int64(10010), f0101_north_america.Deals.TailTransfers[0].TransferedAt.Unix())
	require.Len(t, f0101_north_america.Deals.TailSealed, 1)
	require.Equal(t, 300*60, f0101_north_america.Deals.TailSealed[0].DurationSeconds)
	require.Equal(t, int64(38000), f0101_north_america.Deals.TailSealed[0].SealedAt.Unix())
	// Africa
	f0101_africa := f0101.Textile.Regions["africa"]
	require.Equal(t, 1, f0101_africa.Deals.Total)
	require.Equal(t, int64(311), f0101_africa.Deals.Last.Unix())
	require.Equal(t, 2, f0101_africa.Deals.Failures)                    // Test 2 failures.
	require.Equal(t, int64(313), f0101_africa.Deals.LastFailure.Unix()) // Max of 2 failures.
	require.Len(t, f0101_north_america.Deals.TailTransfers, 1)
	require.Equal(t, float64(0.2), f0101_africa.Deals.TailTransfers[0].MiBPerSec)
	require.Equal(t, int64(10010), f0101_africa.Deals.TailTransfers[0].TransferedAt.Unix())
	require.Len(t, f0101_africa.Deals.TailSealed, 0) // Our record didn't have sealing information.
}

var (
	getRegionalGeneralInfoDuke1SouthAmerica = []model.PowStorageDealRecord{
		{
			Pending:           false,
			DataTransferStart: 10000,
			DataTransferEnd:   10010,
			TransferSize:      1024 * 1024 * 100, // 100MiB in 10 sec =  10MiB/s
			SealingStart:      20000,
			SealingEnd:        23600, // Sealing duration = 60min (1hr)
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "0_1",
				Miner:       "f0100",
			},
			UpdatedAt: 300,
		},
		{
			Pending:           false,
			DataTransferStart: 10000,
			DataTransferEnd:   10002,
			TransferSize:      1024 * 1024 * 50, // 50MiB in 2 sec =  25MiB/s
			SealingStart:      20000,
			SealingEnd:        56000, // Sealing duration = 600 min (10hr)
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "0_2",
				Miner:       "f0100",
			},
			UpdatedAt: 301,
		},
		{
			Pending: false,
			Failed:  true,
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "0_5",
				Miner:       "f0100",
			},
			UpdatedAt: 304,
		},
		{
			Pending: true,
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "1_1",
				Miner:       "f0101",
			},
			UpdatedAt: 310,
		},
	}
	getRegionalGeneralInfoDuke2NorthAmerica = []model.PowStorageDealRecord{
		{
			Pending:           false,
			DataTransferStart: 10000,
			DataTransferEnd:   10010,
			TransferSize:      1024 * 1024 * 10, // 10MiB in 10 sec =  1MiB/s
			SealingStart:      20000,
			SealingEnd:        38000, // Sealing duration = 300 min (5hr)
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "0_3",
				Miner:       "f0100",
			},
			UpdatedAt: 302,
		},
		{
			Pending:           false,
			DataTransferStart: 10000,
			DataTransferEnd:   10010,
			TransferSize:      1024 * 1024 * 2, // 2MiB in 10 sec =  0.2MiB/s
			SealingStart:      20000,
			SealingEnd:        38000, // Sealing duration = 300 min (5hr)
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "1_2",
				Miner:       "f0101",
			},
			UpdatedAt: 310,
		},
	}
	getRegionalGeneralInfoDuke3Africa = []model.PowStorageDealRecord{
		{
			Pending:           false,
			DataTransferStart: 0,
			DataTransferEnd:   0, // Simulate not having transfer times.
			TransferSize:      1024 * 1024 * 1,
			SealingStart:      20000,
			SealingEnd:        29000, // Sealing duration = 150 min (2.5hr)

			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "0_4",
				Miner:       "f0100",
			},
			UpdatedAt: 303,
		},
		{
			Pending:           false,
			DataTransferStart: 10000,
			DataTransferEnd:   10010,
			TransferSize:      1024 * 1024 * 2, // 2MiB in 10 sec =  0.2MiB/s
			SealingStart:      0,
			SealingEnd:        0, // Simulate not having sealing times.

			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "1_3",
				Miner:       "f0101",
			},
			UpdatedAt: 311,
		},
		{
			Pending: false,
			Failed:  true,
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "1_4",
				Miner:       "f0101",
			},
			UpdatedAt: 312,
		},
		{
			Pending: false,
			Failed:  true,
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "1_5",
				Miner:       "f0101",
			},
			UpdatedAt: 313,
		},
	}
)
