package store

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-ds-mongo/test"
	"github.com/textileio/textile/v2/api/mindexd/records"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMain(m *testing.M) {
	cls := test.StartMongoDB()
	ret := m.Run()
	cls()
	os.Exit(ret)
}

func TestGetLastUpdatedAt(t *testing.T) {
	ctx := context.Background()
	db := setup(t, ctx)

	s, err := New(db)
	require.NoError(t, err)

	// Check non-existant last updated at behavior.
	uat, err := s.GetLastStorageDealRecordUpdatedAt(ctx, "none")
	require.NoError(t, err)
	require.Equal(t, int64(0), uat)
	uat, err = s.GetLastRetrievalRecordUpdatedAt(ctx, "none")
	require.NoError(t, err)
	require.Equal(t, int64(0), uat)

	// Insert some records.
	err = s.PersistStorageDealRecords(ctx, "duke-1", testStorageDealRecords)
	require.NoError(t, err)
	err = s.PersistRetrievalRecords(ctx, "duke-1", testRetrievalRecords)
	require.NoError(t, err)

	// Check non-existant last updated at behavior.
	uat, err = s.GetLastStorageDealRecordUpdatedAt(ctx, "duke-1")
	require.NoError(t, err)
	require.Equal(t, testStorageDealRecords[1].UpdatedAt, uat)
	uat, err = s.GetLastRetrievalRecordUpdatedAt(ctx, "duke-1")
	require.NoError(t, err)
	require.Equal(t, testRetrievalRecords[1].UpdatedAt, uat)
}

func setup(t *testing.T, ctx context.Context) *mongo.Database {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(test.GetMongoUri()))
	require.NoError(t, err)
	db := client.Database("test_mindex")
	_ = db.Drop(ctx)
	db = client.Database("test_mindex")
	t.Cleanup(func() {
		err := db.Drop(ctx)
		require.NoError(t, err)
	})
	return db
}

var (
	testStorageDealRecords = []records.PowStorageDealRecord{
		{
			RootCid: "StorageRootCid1",
			Address: "Addr1",
			Pending: true,
			DealInfo: records.PowStorageDealRecordDealInfo{
				ProposalCid:     "SD1",
				StateId:         1,
				StateName:       "StateName1",
				Miner:           "f0100",
				PieceCid:        "StoragePieceCid1",
				Size:            1000,
				PricePerEpoch:   1001,
				StartEpoch:      3000,
				Duration:        23,
				DealId:          10001,
				ActivationEpoch: 499,
				Message:         "msg",
			},
			TransferSize:      1000,
			DataTransferStart: 100,
			DataTransferEnd:   102,
			SealingStart:      200,
			SealingEnd:        204,
			ErrMsg:            "err msg",
			CreatedAt:         80,
			UpdatedAt:         300,
		},
		{
			RootCid: "StorageRootCid2",
			Address: "Addr2",
			Pending: true,
			DealInfo: records.PowStorageDealRecordDealInfo{
				ProposalCid:     "SD2",
				StateId:         1,
				StateName:       "StateName1",
				Miner:           "f0100",
				PieceCid:        "StoragePieceCid2",
				Size:            1000,
				PricePerEpoch:   1001,
				StartEpoch:      3000,
				Duration:        23,
				DealId:          10001,
				ActivationEpoch: 499,
				Message:         "msg",
			},
			TransferSize:      1000,
			DataTransferStart: 100,
			DataTransferEnd:   102,
			SealingStart:      200,
			SealingEnd:        204,
			ErrMsg:            "err msg",
			CreatedAt:         81,
			UpdatedAt:         305,
		},
	}

	testRetrievalRecords = []records.PowRetrievalRecord{
		{
			Address:           "Addr1",
			DataTransferStart: 1003,
			DataTransferEnd:   1004,
			ErrMsg:            "err msg 2",
			CreatedAt:         300,
			UpdatedAt:         502,
			DealInfo: records.PowRetrievalRecordDealInfo{
				RootCid:  "RetrievalRootCid1",
				Size:     3000,
				MinPrice: 329,
				Miner:    "f01002",
			},
		},
		{
			Address:           "Addr1",
			DataTransferStart: 1003,
			DataTransferEnd:   1004,
			ErrMsg:            "err msg 2",
			CreatedAt:         303,
			UpdatedAt:         505,
			DealInfo: records.PowRetrievalRecordDealInfo{
				RootCid:  "RetrievalRootCid1",
				Size:     3000,
				MinPrice: 329,
				Miner:    "f01003",
			},
		},
	}
)
