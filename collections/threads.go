package collections

import (
	"context"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/api/common"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Thread struct {
	ID    thread.ID
	Owner crypto.PubKey
	Name  string
	Key   string
}

type Threads struct {
	col *mongo.Collection
}

func NewThreads(ctx context.Context, db *mongo.Database) (*Threads, error) {
	d := &Threads{col: db.Collection("threads")}
	_, err := d.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"_id.owner", 1}, {"name", 1}},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.D{{"name", bson.M{"$exists": 1}}}),
		},
		{
			Keys: bson.D{{"key_id", 1}},
		},
	})
	return d, err
}

func (t *Threads) Create(ctx context.Context, id thread.ID, owner crypto.PubKey) (*Thread, error) {
	name, _ := common.ThreadNameFromContext(ctx)
	key, _ := common.APIKeyFromContext(ctx)
	doc := &Thread{
		ID:    id,
		Owner: owner,
		Name:  name,
		Key:   key,
	}
	ownerID, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return nil, err
	}
	raw := bson.M{
		"_id":    bson.M{"owner": ownerID, "thread": id.Bytes()},
		"key_id": doc.Key,
	}
	if doc.Name != "" {
		raw["name"] = doc.Name
	}
	if _, err := t.col.InsertOne(ctx, raw); err != nil {
		return nil, err
	}
	return doc, nil
}

func (t *Threads) Get(ctx context.Context, id thread.ID, owner crypto.PubKey) (*Thread, error) {
	ownerID, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return nil, err
	}
	res := t.col.FindOne(ctx, bson.M{"_id.owner": ownerID, "_id.thread": id.Bytes()})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeThread(raw)
}

func (t *Threads) GetByName(ctx context.Context, name string, owner crypto.PubKey) (*Thread, error) {
	ownerID, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return nil, err
	}
	res := t.col.FindOne(ctx, bson.M{"_id.owner": ownerID, "name": name})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeThread(raw)
}

func (t *Threads) List(ctx context.Context, owner crypto.PubKey) ([]Thread, error) {
	ownerID, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return nil, err
	}
	cursor, err := t.col.Find(ctx, bson.M{"_id.owner": ownerID})
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

func (t *Threads) ListByKey(ctx context.Context, key string) ([]Thread, error) {
	cursor, err := t.col.Find(ctx, bson.M{"key_id": key})
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

func (t *Threads) Delete(ctx context.Context, id thread.ID, owner crypto.PubKey) error {
	ownerID, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return err
	}
	res, err := t.col.DeleteOne(ctx, bson.M{"_id.owner": ownerID, "_id.thread": id.Bytes()})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func decodeThread(raw bson.M) (*Thread, error) {
	rid := raw["_id"].(bson.M)
	owner, err := crypto.UnmarshalPublicKey(rid["owner"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	id, err := thread.Cast(rid["thread"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var name string
	if raw["name"] != nil {
		name = raw["name"].(string)
	}
	var key string
	if raw["key_id"] != nil {
		key = raw["key_id"].(string)
	}
	return &Thread{
		ID:    id,
		Owner: owner,
		Name:  name,
		Key:   key,
	}, nil
}
