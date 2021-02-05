package recordstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/textileio/textile/v2/api/mindexd/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrMinerNotExists = errors.New("miner doesn't exists")
)

func (s *Store) GetMinerInfo(ctx context.Context, miner string) (model.MinerInfo, error) {
	filter := bson.M{"_id": miner}
	r := s.idxc.FindOne(ctx, filter)
	if r.Err() == mongo.ErrNoDocuments {
		return model.MinerInfo{}, ErrMinerNotExists
	}

	var mi model.MinerInfo
	if err := r.Decode(&mi); err != nil {
		return model.MinerInfo{}, fmt.Errorf("decoding miner info: %s", err)
	}

	return mi, nil
}

func (s *Store) SummaryCount(ctx context.Context) (int, error) {
	count, err := s.idxc.CountDocuments(ctx, bson.M{})
	if err != nil {
		return 0, fmt.Errorf("counting documents: %s", err)
	}
	return int(count), nil
}
