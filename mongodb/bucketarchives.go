package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type BucketArchive struct {
	BucketKey string   `bson:"_id"`
	Archives  Archives `bson:"archives"`
}

type Archives struct {
	Current Archive   `bson:"current"`
	History []Archive `bson:"history"`
}

type Archive struct {
	Cid        []byte `bson:"cid"`
	JobID      string `bson:"job_id"`
	JobStatus  int    `bson:"job_status"`
	Aborted    bool   `bson:"aborted"`
	AbortedMsg string `bson:"aborted_msg"`
	FailureMsg string `bson:"failure_msg"`
	CreatedAt  int64  `bson:"created_at"`
}

type BucketArchives struct {
	col *mongo.Collection
}

func NewBucketArchives(_ context.Context, db *mongo.Database) (*BucketArchives, error) {
	// ToDo: Let's rename this collection to bucketarchives when there is a data loss event like a powergate reset
	s := &BucketArchives{col: db.Collection("ffsinstances")}
	return s, nil
}

func (k *BucketArchives) Create(ctx context.Context, bucketKey string) error {
	ffs := &BucketArchive{
		BucketKey: bucketKey,
	}
	_, err := k.col.InsertOne(ctx, ffs)
	return err
}

func (k *BucketArchives) Replace(ctx context.Context, ffs *BucketArchive) error {
	res, err := k.col.ReplaceOne(ctx, bson.M{"_id": ffs.BucketKey}, ffs)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (k *BucketArchives) Get(ctx context.Context, bucketKey string) (*BucketArchive, error) {
	res := k.col.FindOne(ctx, bson.M{"_id": bucketKey})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw BucketArchive
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return &raw, nil
}
