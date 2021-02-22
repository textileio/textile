package store

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/textile/v2/api/mindexd/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (s *Store) GetLastIndexSnapshotTime(ctx context.Context) (time.Time, error) {
	opts := options.FindOne()
	opts = opts.SetSort(bson.D{bson.E{Key: "created_at", Value: -1}})
	opts = opts.SetProjection(bson.M{"created_at": 1})
	sr := s.hidxc.FindOne(ctx, bson.M{}, opts)
	if sr.Err() == mongo.ErrNoDocuments {
		// No snapshots?, then return a very old date.
		return time.Time{}, nil
	}
	if sr.Err() != nil {
		return time.Time{}, fmt.Errorf("get last snapshot time: %s", sr.Err())
	}

	var lastSnapshot model.MinerInfoSnapshot
	if err := sr.Decode(&lastSnapshot); err != nil {
		return time.Time{}, fmt.Errorf("decoding snapshot created-at field: %s", err)
	}

	return lastSnapshot.CreatedAt, nil
}

func (s *Store) GenerateMinerIndexSnapshot(ctx context.Context) error {
	pipeline := bson.A{
		bson.M{
			"$project": bson.M{
				"_id":        0,
				"created_at": "$$NOW",
				"miner_info": "$$ROOT",
			},
		},
		bson.M{
			"$merge": bson.M{
				"into": "minerindexhistory",
				"on":   "_id",
			},
		},
	}

	c, err := s.idxc.Aggregate(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("executing pipeline: %s", err)
	}
	c.Close(ctx)

	return nil
}
