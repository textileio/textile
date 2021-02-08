package store

import (
	"context"
	"fmt"

	"github.com/textileio/textile/v2/api/mindexd/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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
