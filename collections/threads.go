package collections

import (
	"context"

	"github.com/textileio/go-threads/core/thread"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Thread struct {
	ID       primitive.ObjectID `bson:"_id"`
	ThreadID thread.ID          `bson:"thread_id"`
	OwnerID  primitive.ObjectID `bson:"owner_id"`
	KeyID    primitive.ObjectID `bson:"key_id"`
	Primary  bool               `bson:"primary"`
}

func NewThreadContext(ctx context.Context, thread *Thread) context.Context {
	return context.WithValue(ctx, ctxKey("thread"), thread)
}

func ThreadFromContext(ctx context.Context) (*Thread, bool) {
	th, ok := ctx.Value(ctxKey("thread")).(*Thread)
	return th, ok
}

type Threads struct {
	col *mongo.Collection
}

func NewThreads(ctx context.Context, db *mongo.Database) (*Threads, error) {
	d := &Threads{col: db.Collection("threads")}
	_, err := d.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"thread_id", 1}, {"owner_id", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{"owner_id", 1}, {"primary", 1}},
		},
	})
	return d, err
}

func (t *Threads) Create(ctx context.Context, threadID thread.ID, ownerID, keyID primitive.ObjectID) (*Thread, error) {
	var primary bool
	res := t.col.FindOne(ctx, bson.M{"owner_id": ownerID, "primary": true})
	if res.Err() != nil {
		primary = true
	}

	doc := &Thread{
		ID:       primitive.NewObjectID(),
		ThreadID: threadID,
		OwnerID:  ownerID,
		KeyID:    keyID,
		Primary:  primary,
	}
	if _, err := t.col.InsertOne(ctx, bson.M{
		"_id":       doc.ID,
		"thread_id": doc.ThreadID.Bytes(),
		"owner_id":  doc.OwnerID,
		"key_id":    doc.KeyID,
		"primary":   doc.Primary,
	}); err != nil {
		return nil, err
	}
	return doc, nil
}

func (t *Threads) Get(ctx context.Context, threadID thread.ID, ownerID primitive.ObjectID) (*Thread, error) {
	res := t.col.FindOne(ctx, bson.M{"thread_id": threadID.Bytes(), "owner_id": ownerID})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeThread(raw)
}

func (t *Threads) GetPrimary(ctx context.Context, ownerID primitive.ObjectID) (*Thread, error) {
	res := t.col.FindOne(ctx, bson.M{"owner_id": ownerID, "primary": true})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeThread(raw)
}

func (t *Threads) SetPrimary(ctx context.Context, threadID thread.ID, ownerID primitive.ObjectID) error {
	res := t.col.FindOne(ctx, bson.M{"thread_id": threadID.Bytes(), "owner_id": ownerID})
	if res.Err() != nil {
		return res.Err()
	}
	if _, err := t.col.UpdateOne(ctx, bson.M{"owner_id": ownerID, "primary": true}, bson.M{"$set": bson.M{"primary": false}}); err != nil {
		return err
	}
	_, err := t.col.UpdateOne(ctx, bson.M{"thread_id": threadID.Bytes(), "owner_id": ownerID}, bson.M{"$set": bson.M{"primary": true}})
	return err
}

func (t *Threads) List(ctx context.Context, ownerID primitive.ObjectID) ([]Thread, error) {
	cursor, err := t.col.Find(ctx, bson.M{"owner_id": ownerID})
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

func (t *Threads) Delete(ctx context.Context, threadID thread.ID, ownerID primitive.ObjectID) error {
	res, err := t.col.DeleteOne(ctx, bson.M{"thread_id": threadID.Bytes(), "owner_id": ownerID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func decodeThread(raw bson.M) (*Thread, error) {
	threadID, err := thread.Cast(raw["thread_id"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	return &Thread{
		ID:       raw["_id"].(primitive.ObjectID),
		ThreadID: threadID,
		OwnerID:  raw["owner_id"].(primitive.ObjectID),
		KeyID:    raw["key_id"].(primitive.ObjectID),
		Primary:  raw["primary"].(bool),
	}, nil
}
