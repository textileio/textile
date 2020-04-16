package collections

import (
	"context"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const keyLen = 16

type Key struct {
	Key    string
	Secret string
	Owner  crypto.PubKey
	Valid  bool
}

type Keys struct {
	col *mongo.Collection
}

func NewKeys(ctx context.Context, db *mongo.Database) (*Keys, error) {
	s := &Keys{col: db.Collection("keys")}
	_, err := s.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{"owner_id", 1}},
		},
	})
	return s, err
}

func (k *Keys) Create(ctx context.Context, owner crypto.PubKey) (*Key, error) {
	doc := &Key{
		Key:    util.MakeToken(keyLen),
		Secret: util.MakeToken(keyLen),
		Owner:  owner,
		Valid:  true,
	}
	ownerID, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return nil, err
	}
	if _, err := k.col.InsertOne(ctx, bson.M{
		"_id":      doc.Key,
		"secret":   doc.Secret,
		"owner_id": ownerID,
		"valid":    doc.Valid,
	}); err != nil {
		return nil, err
	}
	return doc, nil
}

func (k *Keys) Get(ctx context.Context, key string) (*Key, error) {
	res := k.col.FindOne(ctx, bson.M{"_id": key})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeKey(raw)
}

func (k *Keys) List(ctx context.Context, owner crypto.PubKey) ([]Key, error) {
	ownerID, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return nil, err
	}
	cursor, err := k.col.Find(ctx, bson.M{"owner_id": ownerID})
	if err != nil {
		return nil, err
	}
	var docs []Key
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}
		doc, err := decodeKey(raw)
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

func (k *Keys) Invalidate(ctx context.Context, key string) error {
	res, err := k.col.UpdateOne(ctx, bson.M{"_id": key}, bson.M{"$set": bson.M{"valid": false}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func decodeKey(raw bson.M) (*Key, error) {
	owner, err := crypto.UnmarshalPublicKey(raw["owner_id"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	return &Key{
		Key:    raw["_id"].(string),
		Secret: raw["secret"].(string),
		Owner:  owner,
		Valid:  raw["valid"].(bool),
	}, nil
}
