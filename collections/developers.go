package collections

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Developer struct {
	ID        primitive.ObjectID `bson:"_id"`
	Email     string             `bson:"email"`
	StoreID   string             `bson:"store_id"`
	CreatedAt time.Time          `bson:"created_at"`
}

type Developers struct {
	col *mongo.Collection
}

func NewDevelopers(ctx context.Context, db *mongo.Database) (*Developers, error) {
	d := &Developers{col: db.Collection("developers")}
	_, err := d.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"email", 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	return d, err
}

func (d *Developers) GetOrCreate(ctx context.Context, email, storeID string) (*Developer, error) {
	doc := &Developer{
		ID:        primitive.NewObjectID(),
		StoreID:   storeID,
		Email:     email,
		CreatedAt: time.Now(),
	}
	if _, err := d.col.InsertOne(ctx, doc); err != nil {
		if _, ok := err.(mongo.WriteException); ok {
			return d.Get(ctx, email)
		}
		return nil, err
	}
	return doc, nil
}

func (d *Developers) Get(ctx context.Context, email string) (*Developer, error) {
	var doc *Developer
	res := d.col.FindOne(ctx, bson.M{"email": email})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// @todo: Developer must first delete bucket and orgs they own
// @todo: Delete associated sessions, store, remove from orgs
func (d *Developers) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := d.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
