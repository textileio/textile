package store

import (
	"context"
	"fmt"

	"github.com/textileio/textile/v2/api/mindexd/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Store struct {
	idxc  *mongo.Collection
	hidxc *mongo.Collection
}

func New(db *mongo.Database) (*Store, error) {
	s := &Store{
		idxc:  db.Collection("miner_index"),
		hidxc: db.Collection("history_miner_index"),
	}

	return s, nil
}

func (s *Store) PutTextileInfo(ctx context.Context, miner string, info model.TextileInfo) error {
	filter := bson.M{"_id": miner}
	update := bson.M{"$set": bson.M{"textile": info}}
	opts := options.Update().SetUpsert(true)
	_, err := s.idxc.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("put textile info in mongo: %s", err)
	}

	return nil
}

func (s *Store) PutFilecoinInfo(ctx context.Context, miner string, info model.FilecoinInfo) error {
	filter := bson.M{"_id": miner}
	update := bson.M{"$set": bson.M{"filecoin": info}}
	opts := options.Update().SetUpsert(true)
	_, err := s.idxc.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("put filecoin info in mongo: %s", err)
	}

	return nil
}
