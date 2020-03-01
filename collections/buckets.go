package collections

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Bucket struct {
	ID        primitive.ObjectID `bson:"_id"`
	OwnerID   primitive.ObjectID `bson:"owner_id"`
	Name      string             `bson:"name"`
	EntityID  string             `bson:"entity_id"`
	Address   string             `bson:"address"`
	CreatedAt time.Time          `bson:"created_at"`
}

type Buckets struct {
	col *mongo.Collection
}

func NewBuckets(ctx context.Context, db *mongo.Database) (*Buckets, error) {
	t := &Buckets{col: db.Collection("buckets")}
	_, err := t.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"owner_id", 1}, {"name", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{"entity_id", 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	return t, err
}

func (b *Buckets) Create(ctx context.Context, ownerID primitive.ObjectID, name, entityID, addr string) (*Bucket, error) {
	validName, err := toValidName(name)
	if err != nil {
		return nil, err
	}
	doc := &Bucket{
		ID:        primitive.NewObjectID(),
		OwnerID:   ownerID,
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

func (b *Buckets) Get(ctx context.Context, ownerID primitive.ObjectID, name string) (*Bucket, error) {
	var doc *Bucket
	res := b.col.FindOne(ctx, bson.M{"owner_id": ownerID, "name": name})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (b *Buckets) List(ctx context.Context, ownerID primitive.ObjectID) ([]Bucket, error) {
	cursor, err := b.col.Find(ctx, bson.M{"owner_id": ownerID})
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
