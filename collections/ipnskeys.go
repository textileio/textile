package collections

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	mbase "github.com/multiformats/go-multibase"
	"github.com/textileio/go-threads/core/thread"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type IPNSKey struct {
	ID        string
	Key       crypto.PrivKey
	ThreadID  thread.ID
	CreatedAt time.Time
}

type IPNSKeys struct {
	col *mongo.Collection
}

func NewIPNSKeys(_ context.Context, db *mongo.Database) (*IPNSKeys, error) {
	return &IPNSKeys{col: db.Collection("ipnskeys")}, nil
}

func (k *IPNSKeys) Create(ctx context.Context, threadID thread.ID) (*IPNSKey, error) {
	sk, pk, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	bytes, err := crypto.MarshalPublicKey(pk)
	if err != nil {
		return nil, err
	}
	id, err := mbase.Encode(mbase.Base32, bytes)
	if err != nil {
		return nil, err
	}
	doc := &IPNSKey{
		ID:        id,
		Key:       sk,
		ThreadID:  threadID,
		CreatedAt: time.Now(),
	}
	key, err := crypto.MarshalPrivateKey(sk)
	if err != nil {
		return nil, err
	}
	if _, err := k.col.InsertOne(ctx, bson.M{
		"_id":        id,
		"key":        key,
		"thread_id":  threadID.Bytes(),
		"created_at": doc.CreatedAt,
	}); err != nil {
		return nil, err
	}
	return doc, nil
}

func (k *IPNSKeys) Get(ctx context.Context, id string) (*IPNSKey, error) {
	res := k.col.FindOne(ctx, bson.M{"_id": id})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeIPNSKey(raw)
}

func (k *IPNSKeys) Delete(ctx context.Context, id string) error {
	res, err := k.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func decodeIPNSKey(raw bson.M) (*IPNSKey, error) {
	key, err := crypto.UnmarshalPrivateKey(raw["key"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	threadID, err := thread.Cast(raw["thread_id"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var created time.Time
	if v, ok := raw["created_at"]; ok {
		created = v.(primitive.DateTime).Time()
	}
	return &IPNSKey{
		ID:        raw["_id"].(string),
		Key:       key,
		ThreadID:  threadID,
		CreatedAt: created,
	}, nil
}
