package collections

import (
	"context"

	"github.com/textileio/go-threads/core/thread"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Thread struct {
	ID      thread.ID          `bson:"_id"`
	OwnerID primitive.ObjectID `bson:"owner_id"`
	KeyID   primitive.ObjectID `bson:"key_id"`
}

func NewThreadContext(ctx context.Context, session *Session) context.Context {
	return context.WithValue(ctx, ctxKey("session"), session)
}

func ThreadFromContext(ctx context.Context) (*Thread, bool) {
	db, ok := ctx.Value(ctxKey("db")).(*Thread)
	return db, ok
}

type Threads struct {
	col *mongo.Collection
}

func NewThreads(ctx context.Context, db *mongo.Database) (*Threads, error) {
	d := &Threads{col: db.Collection("threads")}
	_, err := d.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{"owner_id", 1}},
		},
	})
	return d, err
}

func (d *Threads) Create(ctx context.Context, ownerID, keyID primitive.ObjectID) (*Thread, error) {
	doc := &Thread{
		ID:      thread.NewIDV1(thread.Raw, 32),
		OwnerID: ownerID,
		KeyID:   keyID,
	}
	if _, err := d.col.InsertOne(ctx, bson.M{
		"_id":      doc.ID.Bytes(),
		"owner_id": doc.OwnerID,
		"key_id":   doc.KeyID,
	}); err != nil {
		return nil, err
	}
	return doc, nil
}

func (d *Threads) Get(ctx context.Context, id thread.ID) (*Thread, error) {
	res := d.col.FindOne(ctx, bson.M{"_id": id.Bytes()})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeThread(raw)
}

func (d *Threads) List(ctx context.Context, ownerID primitive.ObjectID) ([]Thread, error) {
	cursor, err := d.col.Find(ctx, bson.M{"owner_id": ownerID})
	if err != nil {
		return nil, err
	}
	var docs []Thread
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}
		doc, err := decodeThread(raw)
		if err != nil {
			return nil, err
		}
		docs = append(docs, *doc)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return docs, nil
}

func (d *Threads) Delete(ctx context.Context, id thread.ID) error {
	res, err := d.col.DeleteOne(ctx, bson.M{"_id": id.Bytes()})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func decodeThread(raw bson.M) (*Thread, error) {
	id, err := thread.Cast(raw["_id"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	return &Thread{
		ID:      id,
		OwnerID: raw["owner_id"].(primitive.ObjectID),
		KeyID:   raw["key_id"].(primitive.ObjectID),
	}, nil
}
