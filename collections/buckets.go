package collections

import (
	"context"
	"time"

	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Bucket struct {
	ID        primitive.ObjectID `bson:"_id"`
	Owner     string             `bson:"owner"`
	Name      string             `bson:"name"`
	EntityID  string             `bson:"entity_id"`
	Address   string             `bson:"address"`
	CreatedAt time.Time          `bson:"created_at"`
}

var bucketKey key

func NewBucketContext(ctx context.Context, buck *Bucket) context.Context {
	return context.WithValue(ctx, bucketKey, buck)
}

func BucketFromContext(ctx context.Context) (*Bucket, bool) {
	buck, ok := ctx.Value(devKey).(*Bucket)
	return buck, ok
}

type Buckets struct {
	col *mongo.Collection
}

func NewBuckets(ctx context.Context, db *mongo.Database) (*Buckets, error) {
	t := &Buckets{col: db.Collection("buckets")}
	_, err := t.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"owner", 1}, {"name", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{"entity_id", 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	return t, err
}

func (b *Buckets) Create(ctx context.Context, owner, name, entityID, addr string) (*Bucket, error) {
	validName, err := util.ToValidName(name)
	if err != nil {
		return nil, err
	}
	doc := &Bucket{
		ID:        primitive.NewObjectID(),
		Owner:     owner,
		Name:      validName,
		EntityID:  entityID,
		Address:   addr,
		CreatedAt: time.Now(),
	}
	res, err := b.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}
	doc.ID = res.InsertedID.(primitive.ObjectID)
	return doc, nil
}

func (b *Buckets) Get(ctx context.Context, owner string, name string) (*Bucket, error) {
	var doc *Bucket
	res := b.col.FindOne(ctx, bson.M{"owner": owner, "name": name})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (b *Buckets) List(ctx context.Context, owner string) ([]Bucket, error) {
	cursor, err := b.col.Find(ctx, bson.M{"owner": owner})
	if err != nil {
		return nil, err
	}
	var docs []Bucket
	for cursor.Next(ctx) {
		var doc Bucket
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

func (b *Buckets) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := b.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
