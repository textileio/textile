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

type Message struct {
	ID        string
	From      crypto.PubKey
	To        crypto.PubKey
	Body      []byte
	Read      bool
	CreatedAt time.Time
}

type Messages struct {
	col *mongo.Collection
}

func NewMessages(ctx context.Context, db *mongo.Database) (*Messages, error) {
	i := &Messages{col: db.Collection("messages")}
	_, err := i.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{"from_id", 1}},
		},
		{
			Keys: bson.D{{"to_id", 1}},
		},
		{
			Keys: bson.D{{"read", 1}},
		},
		{
			Keys: bson.D{{"created_at", 1}},
		},
	})
	return i, err
}

func (m *Messages) Create(ctx context.Context, from, to crypto.PubKey, body []byte) (*Message, error) {

	doc := &Message{
		ID:        util.MakeToken(tokenLen),
		From:      from,
		To:        to,
		Body:      body,
		CreatedAt: time.Now(),
		Read:      false,
	}
	fromID, err := crypto.MarshalPublicKey(from)
	if err != nil {
		return nil, err
	}
	toID, err := crypto.MarshalPublicKey(to)
	if err != nil {
		return nil, err
	}
	if _, err := m.col.InsertOne(ctx, bson.M{
		"_id":        doc.ID,
		"from_id":    fromID,
		"to_id":      toID,
		"body":       doc.Body,
		"read":       doc.Read,
		"created_at": doc.CreatedAt,
	}); err != nil {
		return nil, err
	}
	return doc, nil
}

func (m *Messages) Get(ctx context.Context, id string) (*Message, error) {
	res := m.col.FindOne(ctx, bson.M{"_id": id})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeMessage(raw)
}

func (m *Messages) ListInbox(ctx context.Context, key crypto.PubKey) ([]Message, error) {
	toID, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return nil, err
	}
	cursor, err := m.col.Find(ctx, bson.M{"to_id": toID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	return m.list(ctx, cursor)
}

func (m *Messages) ListOutbox(ctx context.Context, key crypto.PubKey) ([]Message, error) {
	toID, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return nil, err
	}
	cursor, err := m.col.Find(ctx, bson.M{"from_id": toID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	return m.list(ctx, cursor)
}

func (m *Messages) list(ctx context.Context, cursor *mongo.Cursor) ([]Message, error) {
	var docs []Message
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}
		doc, err := decodeMessage(raw)
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

func (m *Messages) Read(ctx context.Context, id string) error {
	res, err := m.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"read": true}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (m *Messages) Delete(ctx context.Context, id string) error {
	res, err := m.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (m *Messages) DeleteInbox(ctx context.Context, key crypto.PubKey) error {
	toID, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return err
	}
	_, err = m.col.DeleteMany(ctx, bson.M{"to_id": toID})
	return err
}

func (m *Messages) DeleteOutbox(ctx context.Context, key crypto.PubKey) error {
	fromID, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return err
	}
	_, err = m.col.DeleteMany(ctx, bson.M{"from_id": fromID})
	return err
}

func decodeMessage(raw bson.M) (*Message, error) {
	from, err := crypto.UnmarshalPublicKey(raw["from_id"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	to, err := crypto.UnmarshalPublicKey(raw["to_id"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var created time.Time
	if v, ok := raw["created_at"]; ok {
		created = v.(primitive.DateTime).Time()
	}
	return &Message{
		ID:        raw["_id"].(string),
		From:      from,
		To:        to,
		Body:      raw["body"].(primitive.Binary).Data,
		Read:      raw["read"].(bool),
		CreatedAt: created,
	}, nil
}
