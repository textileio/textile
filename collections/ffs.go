package collections

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type FFSInstance struct {
	BucketKey string `bson:"_id"`
	FFSToken  string
	Archives  Archives
}

type Archives struct {
	Current Archive
	History []Archive
}

type Archive struct {
	Cid       string
	JobID     string
	JobStatus int
	CreatedAt int64
}

type FFSInstances struct {
	col *mongo.Collection
}

func NewFFSInstances(ctx context.Context, db *mongo.Database) (*FFSInstances, error) {
	s := &FFSInstances{col: db.Collection("ffsinstances")}
	return s, nil
}

func (k *FFSInstances) Create(ctx context.Context, bucketKey, ffsToken string) error {
	ffs := &FFSInstance{
		BucketKey: bucketKey,
		FFSToken:  ffsToken,
	}
	_, err := k.col.InsertOne(ctx, ffs)
	return err
}

func (k *FFSInstances) Replace(ctx context.Context, ffs *FFSInstance) error {
	res, err := k.col.ReplaceOne(ctx, bson.M{"_id": ffs.BucketKey}, ffs)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (k *FFSInstances) Get(ctx context.Context, bucketKey string) (*FFSInstance, error) {
	res := k.col.FindOne(ctx, bson.M{"_id": bucketKey})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw FFSInstance
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return &raw, nil
}
