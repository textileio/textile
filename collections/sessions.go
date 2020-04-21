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
	sessionDur = time.Hour * 24 * 7 * 30 * 6
)

type Session struct {
	ID        string
	Owner     crypto.PubKey
	ExpiresAt time.Time
}

func NewSessionContext(ctx context.Context, session *Session) context.Context {
	return context.WithValue(ctx, ctxKey("session"), session)
}

func SessionFromContext(ctx context.Context) (*Session, bool) {
	session, ok := ctx.Value(ctxKey("session")).(*Session)
	return session, ok
}

type Sessions struct {
	col *mongo.Collection
}

func NewSessions(ctx context.Context, db *mongo.Database) (*Sessions, error) {
	s := &Sessions{col: db.Collection("sessions")}
	_, err := s.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{"developer_id", 1}},
		},
	})
	return s, err
}

func (s *Sessions) Create(ctx context.Context, owner crypto.PubKey) (*Session, error) {
	doc := &Session{
		ID:        util.MakeToken(tokenLen),
		Owner:     owner,
		ExpiresAt: time.Now().Add(sessionDur),
	}
	ownerID, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return nil, err
	}
	if _, err := s.col.InsertOne(ctx, bson.M{
		"_id":        doc.ID,
		"owner_id":   ownerID,
		"expires_at": doc.ExpiresAt,
	}); err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *Sessions) Get(ctx context.Context, id string) (*Session, error) {
	res := s.col.FindOne(ctx, bson.M{"_id": id})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeSession(raw)
}

func (s *Sessions) Touch(ctx context.Context, id string) error {
	expiry := time.Now().Add(sessionDur)
	res, err := s.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"expires_at": expiry}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (s *Sessions) Delete(ctx context.Context, id string) error {
	res, err := s.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func decodeSession(raw bson.M) (*Session, error) {
	owner, err := crypto.UnmarshalPublicKey(raw["owner_id"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var expiry time.Time
	if v, ok := raw["expires_at"]; ok {
		expiry = v.(primitive.DateTime).Time()
	}
	return &Session{
		ID:        raw["_id"].(string),
		Owner:     owner,
		ExpiresAt: expiry,
	}, nil
}
