package store

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/textile/v2/api/mindexd/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *Store) PutFilecoinInfo(ctx context.Context, miner string, info model.FilecoinInfo) error {
	info.UpdatedAt = time.Now()
	filter := bson.M{"_id": miner}
	update := bson.M{"$set": bson.M{"filecoin": info}}
	opts := options.Update().SetUpsert(true)
	_, err := s.idxc.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("put filecoin-info in collection: %s", err)
	}

	return nil
}

func (s *Store) PutMetadataLocation(ctx context.Context, miner string, location string) error {
	filter := bson.M{"_id": miner}
	update := bson.M{"$set": bson.M{"metadata.location": location}}
	opts := options.Update().SetUpsert(true)
	_, err := s.idxc.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("put metadata location: %s", err)
	}

	return nil
}
