package recordstore

import (
	"context"
	"encoding/json"
	"fmt"
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
	buf, _ := json.MarshalIndent(f0100, "", " ")
	fmt.Printf("H: %s\n", buf)
	// South-america.
	require.Equal(t, 2, f0100.Textile.Regions["south_america"].Deals.Total)
	require.Equal(t, int64(301), f0100.Textile.Regions["south_america"].Deals.Last.Unix())
	require.Equal(t, 1, f0100.Textile.Regions["south_america"].Deals.Failures)
	require.Equal(t, int64(304), f0100.Textile.Regions["south_america"].Deals.LastFailure.Unix())
	// North-america.
	require.Equal(t, 1, f0100.Textile.Regions["north_america"].Deals.Total)
	require.Equal(t, int64(302), f0100.Textile.Regions["north_america"].Deals.Last.Unix())
	require.Equal(t, 0, f0100.Textile.Regions["north_america"].Deals.Failures)
	require.True(t, f0100.Textile.Regions["north_america"].Deals.LastFailure.IsZero())
	// Africa
	require.Equal(t, 1, f0100.Textile.Regions["africa"].Deals.Total)
	require.Equal(t, int64(303), f0100.Textile.Regions["africa"].Deals.Last.Unix())
	require.Equal(t, 0, f0100.Textile.Regions["north_america"].Deals.Failures)
	require.True(t, f0100.Textile.Regions["north_america"].Deals.LastFailure.IsZero())

	/* TTODO
	// f0101
	require.Len(t, r["f0101"], 2) // Two regions, since in one is still pending so doesn't count.

	require.Equal(t, 1, r["f0101"]["north_america"].Total) // 1 in north_america
	require.Equal(t, int64(310), r["f0101"]["north_america"].Last)

	require.Equal(t, 1, r["f0101"]["africa"].Total) // 1 in africa
	require.Equal(t, int64(311), r["f0101"]["africa"].Last)
	*/
}

var (
	getRegionalGeneralInfoDuke1SouthAmerica = []model.PowStorageDealRecord{
		{
			Pending: false,
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "0_1",
				Miner:       "f0100",
			},
			UpdatedAt: 300,
		},
		{
			Pending: false,
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
			Pending: false,
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "0_3",
				Miner:       "f0100",
			},
			UpdatedAt: 302,
		},
		{
			Pending: false,
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "1_2",
				Miner:       "f0101",
			},
			UpdatedAt: 310,
		},
	}
	getRegionalGeneralInfoDuke3Africa = []model.PowStorageDealRecord{
		{
			Pending: false,
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "0_4",
				Miner:       "f0100",
			},
			UpdatedAt: 303,
		},
		{
			Pending: false,
			DealInfo: model.PowStorageDealRecordDealInfo{
				ProposalCid: "1_3",
				Miner:       "f0101",
			},
			UpdatedAt: 311,
		},
	}
)
