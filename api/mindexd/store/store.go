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
	sdrc *mongo.Collection
	rrc  *mongo.Collection
	idxc *mongo.Collection
}

func New(db *mongo.Database) (*Store, error) {
	s := &Store{
		sdrc: db.Collection("storagedealrecords"),
		rrc:  db.Collection("retrievalrecords"),
		idxc: db.Collection("miner_index"),
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

	return nil
}
