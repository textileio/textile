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

type Developer struct {
	ID        primitive.ObjectID `bson:"_id"`
	Username  string             `bson:"username"`
	Email     string             `bson:"email"`
	StoreID   string             `bson:"store_id"`
	CreatedAt time.Time          `bson:"created_at"`
}

var devKey key

func NewDevContext(ctx context.Context, dev *Developer) context.Context {
	return context.WithValue(ctx, devKey, dev)
}

func DevFromContext(ctx context.Context) (*Developer, bool) {
	dev, ok := ctx.Value(devKey).(*Developer)
	return dev, ok
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
		{
			Keys:    bson.D{{"username", 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	return d, err
}

func (d *Developers) GetOrCreate(ctx context.Context, username, email string) (*Developer, error) {
	validUsername, err := util.ToValidName(username)
	if err != nil {
		return nil, err
	}
	doc := &Developer{
		ID:        primitive.NewObjectID(),
		Username:  validUsername,
		Email:     email,
		CreatedAt: time.Now(),
	}
	if _, err := d.col.InsertOne(ctx, doc); err != nil {
		if _, ok := err.(mongo.WriteException); ok {
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
		return nil, err
	}
	return doc, nil
}

func (d *Developers) Get(ctx context.Context, id primitive.ObjectID) (*Developer, error) {
	var doc *Developer
	res := d.col.FindOne(ctx, bson.M{"_id": id})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (d *Developers) ListMembers(ctx context.Context, members []Member) ([]Developer, error) {
	ids := make([]primitive.ObjectID, len(members))
	for i, m := range members {
		ids[i] = m.ID
	}
	cursor, err := d.col.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	var docs []Developer
	for cursor.Next(ctx) {
		var doc Developer
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
