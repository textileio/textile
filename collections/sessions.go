package collections

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	sessionTokenLen = 44
	sessionDur      = time.Hour * 24 * 7 * 30
)

type Session struct {
	ID          primitive.ObjectID `bson:"_id"`
	DeveloperID primitive.ObjectID `bson:"developer_id"`
	Token       string             `bson:"token"`
	ExpiresAt   time.Time          `bson:"expires_at"`
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
		{
			Keys: bson.D{{"token", 1}},
		},
	})
	return s, err
}

func (s *Sessions) Create(ctx context.Context, developerID primitive.ObjectID) (*Session, error) {
	token, err := makeStringToken(sessionTokenLen)
	if err != nil {
		return nil, err
	}
	doc := &Session{
		ID:          primitive.NewObjectID(),
		DeveloperID: developerID,
		Token:       token,
		ExpiresAt:   time.Now().Add(sessionDur),
	}
	res, err := s.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}
	doc.ID = res.InsertedID.(primitive.ObjectID)
	return doc, nil
}

func (s *Sessions) Get(ctx context.Context, token string) (*Session, error) {
	var doc *Session
	res := s.col.FindOne(ctx, bson.M{"token": token})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *Sessions) Touch(ctx context.Context, token string) error {
	expiry := time.Now().Add(sessionDur)
	res, err := s.col.UpdateOne(ctx, bson.M{"token": token}, bson.M{"$set": bson.M{"expires_at": expiry}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (s *Sessions) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := s.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
