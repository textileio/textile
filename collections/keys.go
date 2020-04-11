package collections

import (
	"context"

	sym "github.com/textileio/go-threads/crypto/symmetric"
	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Key struct {
	ID      primitive.ObjectID `bson:"_id"`
	OwnerID primitive.ObjectID `bson:"owner_id"`
	Token   string             `bson:"token"`
	Secret  []byte             `bson:"secret"`
}

func NewKeyContext(ctx context.Context, key *Key) context.Context {
	return context.WithValue(ctx, ctxKey("key"), key)
}

func KeyFromContext(ctx context.Context) (*Key, bool) {
	key, ok := ctx.Value(ctxKey("key")).(*Key)
	return key, ok
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
		{
			Keys: bson.D{{"token", 1}},
		},
	})
	return s, err
}

func (s *Keys) Create(ctx context.Context, ownerID primitive.ObjectID) (*Key, error) {
	doc := &Key{
		ID:      primitive.NewObjectID(),
		OwnerID: ownerID,
		Token:   util.MakeToken(tokenLen),
		Secret:  sym.New().Bytes(),
	}
	res, err := s.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}
	doc.ID = res.InsertedID.(primitive.ObjectID)
	return doc, nil
}

func (s *Keys) Get(ctx context.Context, token string) (*Key, error) {
	var doc *Key
	res := s.col.FindOne(ctx, bson.M{"token": token})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *Keys) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := s.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
