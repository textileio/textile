package collections

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	bucketTokenLen = 44
)

type BucketToken struct {
	ID        primitive.ObjectID `bson:"_id"`
	BucketID  primitive.ObjectID `bson:"bucket_id"`
	Token     string             `bson:"token"`
	CreatedAt time.Time          `bson:"created_at"`
}

type BucketTokens struct {
	col *mongo.Collection
}

func NewBucketTokens(ctx context.Context, db *mongo.Database) (*BucketTokens, error) {
	b := &BucketTokens{col: db.Collection("bucketTokens")}
	_, err := b.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{"bucket_id", 1}},
		},
		{
			Keys: bson.D{{"token", 1}},
		},
	})
	return b, err
}

func (b *BucketTokens) Create(ctx context.Context, bucketID primitive.ObjectID) (*BucketToken, error) {
	token, err := makeStringToken(bucketTokenLen)
	if err != nil {
		return nil, err
	}
	doc := &BucketToken{
		ID:        primitive.NewObjectID(),
		BucketID:  bucketID,
		Token:     token,
		CreatedAt: time.Now(),
	}
	res, err := b.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}
	doc.ID = res.InsertedID.(primitive.ObjectID)
	return doc, nil
}

func (b *BucketTokens) Get(ctx context.Context, token string) (*BucketToken, error) {
	var doc *BucketToken
	res := b.col.FindOne(ctx, bson.M{"token": token})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (b *BucketTokens) List(ctx context.Context, bucketID primitive.ObjectID) ([]BucketToken, error) {
	cursor, err := b.col.Find(ctx, bson.M{"bucket_id": bucketID})
	if err != nil {
		return nil, err
	}
	var docs []BucketToken
	for cursor.Next(ctx) {
		var doc BucketToken
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return docs, nil
}

func (b *BucketTokens) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := b.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
