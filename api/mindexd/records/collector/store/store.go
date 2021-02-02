package store

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Store struct {
	sdrc *mongo.Collection
	rrc  *mongo.Collection
}

func New(db *mongo.Database) (*Store, error) {
	s := &Store{
		sdrc: db.Collection("storagedealrecords"),
		rrc:  db.Collection("retrievalrecords"),
	}
	if err := s.ensureIndexes(); err != nil {
		return nil, fmt.Errorf("ensuring mongodb indexes: %s", err)
	}

	return s, nil
}

func (s *Store) GetLastStorageDealRecordUpdatedAt(ctx context.Context, powName string) (int64, error) {
	filter := bson.M{"pow_name": powName}
	opts := options.Find()
	opts = opts.SetSort(bson.D{{Key: "pow_storage_deal_record.updated_at", Value: -1}})
	opts = opts.SetLimit(1)
	opts = opts.SetProjection(bson.M{"pow_storage_deal_record.updated_at": 1})
	c, err := s.sdrc.Find(ctx, filter, opts)
	if err != nil {
		return 0, fmt.Errorf("executing find: %s", err)
	}

	var res *[]powStorageDealRecord
	if err := c.All(ctx, res); err != nil {
		return 0, fmt.Errorf("fetching find result: %s", err)
	}
	if len(*res) == 0 {
		// If we don't have any records, just return a very old lastUpdatedAt
		// to let it start from the beginning.
		return 0, nil
	}

	return (*res)[0].PowStorageDealRecord.UpdatedAt, nil
}

func (s *Store) GetLastRetrievalRecordUpdatedAt(ctx context.Context, powName string) (int64, error) {
	filter := bson.M{"pow_name": powName}
	opts := options.Find()
	opts = opts.SetSort(bson.D{{Key: "pow_retrieval_record.updated_at", Value: -1}})
	opts = opts.SetLimit(1)
	opts = opts.SetProjection(bson.M{"pow_retrieval_record.updated_at": 1})
	c, err := s.sdrc.Find(ctx, filter, opts)
	if err != nil {
		return 0, fmt.Errorf("executing find: %s", err)
	}

	var res *[]powRetrievalRecord
	if err := c.All(ctx, res); err != nil {
		return 0, fmt.Errorf("fetching find result: %s", err)
	}
	if len(*res) == 0 {
		// If we don't have any records, just return a very old lastUpdatedAt
		// to let it start from the beginning.
		return 0, nil
	}

	return (*res)[0].PowRetrievalRecord.UpdatedAt, nil
}

func (s *Store) ensureIndexes() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, err := s.sdrc.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{primitive.E{Key: "username", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetCollation(&options.Collation{Locale: "en", Strength: 2}).
				SetSparse(true),
		},
		{
			Keys: bson.D{primitive.E{Key: "email", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetSparse(true),
		},
		{
			Keys: bson.D{primitive.E{Key: "members._id", Value: 1}},
		},
	})
	if err != nil {
		return fmt.Errorf("creating storage-deal records index: %s", err)
	}

	return nil
}
