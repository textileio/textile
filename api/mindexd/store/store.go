package store

import (
	"context"
	"fmt"
	"time"

	logger "github.com/ipfs/go-log/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	log = logger.Logger("store")
)

type Store struct {
	sdrc  *mongo.Collection
	rrc   *mongo.Collection
	idxc  *mongo.Collection
	hidxc *mongo.Collection
	ptc   *mongo.Collection
}

func New(db *mongo.Database) (*Store, error) {
	s := &Store{
		sdrc:  db.Collection("storagedealrecords"),
		rrc:   db.Collection("retrievalrecords"),
		idxc:  db.Collection("minerindex"),
		hidxc: db.Collection("minerindexhistory"),
		ptc:   db.Collection("powtargets"),
	}
	if err := s.ensureIndexes(); err != nil {
		return nil, fmt.Errorf("ensuring mongodb indexes: %s", err)
	}

	return s, nil
}

func (s *Store) ensureIndexes() error {
	// StorageDealRecords indexes
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, err := s.sdrc.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "pow_name", Value: 1},
				bson.E{Key: "pow_storage_deal_record.updated_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				bson.E{Key: "pow_storage_deal_record.deal_info.miner", Value: 1},
				bson.E{Key: "region", Value: 1},
				bson.E{Key: "pow_storage_deal_record.pending", Value: 1},
				bson.E{Key: "pow_storage_deal_record.failed", Value: 1},
				bson.E{Key: "pow_storage_deal_record.updated_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				bson.E{Key: "pow_storage_deal_record.pending", Value: 1},
				bson.E{Key: "pow_storage_deal_record.deal_info.miner", Value: 1},
				bson.E{Key: "region", Value: 1},
				bson.E{Key: "pow_storage_deal_record.failed", Value: 1},
				bson.E{Key: "pow_storage_deal_record.updated_at", Value: -1},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("creating storage-deal records index: %s", err)
	}

	// RetrievalRecords indexes
	_, err = s.rrc.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{
				bson.E{Key: "pow_name", Value: 1},
				bson.E{Key: "pow_retrieval_record.updated_at", Value: -1},
			},
		},
		{
			Keys: bson.D{
				bson.E{Key: "pow_retrieval_record.deal_info.miner", Value: 1},
				bson.E{Key: "region", Value: 1},
				bson.E{Key: "pow_retrieval_record.failed", Value: 1},
				bson.E{Key: "pow_retrieval_record.updated_at", Value: -1},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("creating retrieval records index: %s", err)
	}

	// Index indexes
	_, err = s.idxc.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{bson.E{Key: "metadata.location", Value: 1}},
		},
		{
			Keys: bson.D{bson.E{Key: "filecoin.ask_price", Value: 1}},
		},
		{
			Keys: bson.D{bson.E{Key: "filecoin.ask_verified_price", Value: 1}},
		},
		{
			Keys: bson.D{bson.E{Key: "filecoin.active_sectors", Value: -1}},
		},
		{
			Keys: bson.D{bson.E{Key: "textile.deals_summary.total", Value: -1}},
		},
		{
			Keys: bson.D{bson.E{Key: "textile.deals_summary.last", Value: -1}},
		},
		{
			Keys: bson.D{bson.E{Key: "textile.retrievals_summary.total", Value: -1}},
		},
		{
			Keys: bson.D{bson.E{Key: "textile.retrievals_summary.last", Value: -1}},
		},
		{
			Keys: bson.D{bson.E{Key: "textile.region.021.deals.total", Value: -1}},
		},
		{
			Keys: bson.D{bson.E{Key: "textile.region.021.deals.last", Value: -1}},
		},
		{
			Keys: bson.D{bson.E{Key: "textile.region.021.retrievals.total", Value: -1}},
		},
		{
			Keys: bson.D{bson.E{Key: "textile.region.021.retrievals.last", Value: -1}},
		},
	})
	if err != nil {
		return fmt.Errorf("creating retrieval records index: %s", err)
	}

	return nil
}
