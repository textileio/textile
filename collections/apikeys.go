package collections

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	keyLen    = 16
	secretLen = 24
)

type APIKey struct {
	Key       string
	Secret    string
	Owner     crypto.PubKey
	Valid     bool
	CreatedAt time.Time
}

type APIKeys struct {
	col *mongo.Collection
}

func NewAPIKeys(ctx context.Context, db *mongo.Database) (*APIKeys, error) {
	s := &APIKeys{col: db.Collection("apikeys")}
	_, err := s.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{"owner_id", 1}},
		},
	})
	return s, err
}

func (k *APIKeys) Create(ctx context.Context, owner crypto.PubKey) (*APIKey, error) {
	doc := &APIKey{
		Key:       util.MakeToken(keyLen),
		Secret:    util.MakeToken(secretLen),
		Owner:     owner,
		Valid:     true,
		CreatedAt: time.Now(),
	}
	ownerID, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return nil, err
	}
	if _, err := k.col.InsertOne(ctx, bson.M{
		"_id":        doc.Key,
		"secret":     doc.Secret,
		"owner_id":   ownerID,
		"valid":      doc.Valid,
		"created_at": doc.CreatedAt,
	}); err != nil {
		return nil, err
	}
	return doc, nil
}

func (k *APIKeys) Get(ctx context.Context, key string) (*APIKey, error) {
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

func (k *APIKeys) List(ctx context.Context, owner crypto.PubKey) ([]APIKey, error) {
	ownerID, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return nil, err
	}
	cursor, err := k.col.Find(ctx, bson.M{"owner_id": ownerID})
	if err != nil {
		return nil, err
	}
	var docs []APIKey
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

func (k *APIKeys) Invalidate(ctx context.Context, key string) error {
	res, err := k.col.UpdateOne(ctx, bson.M{"_id": key}, bson.M{"$set": bson.M{"valid": false}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func decodeKey(raw bson.M) (*APIKey, error) {
	owner, err := crypto.UnmarshalPublicKey(raw["owner_id"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var created time.Time
	if v, ok := raw["created_at"]; ok {
		created = v.(primitive.DateTime).Time()
	}
	return &APIKey{
		Key:       raw["_id"].(string),
		Secret:    raw["secret"].(string),
		Owner:     owner,
		Valid:     raw["valid"].(bool),
		CreatedAt: created,
	}, nil
}
