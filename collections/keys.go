package collections

import (
	"context"

	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const keyLen = 16

type Key struct {
	ID      primitive.ObjectID `bson:"_id"`
	OwnerID primitive.ObjectID `bson:"owner_id"`
	Token   string             `bson:"token"`
	Secret  string             `bson:"secret"`
	Valid   bool               `bson:"valid"`
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
			Keys:    bson.D{{"token", 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	return s, err
}

func (k *Keys) Create(ctx context.Context, ownerID primitive.ObjectID) (*Key, error) {
	doc := &Key{
		ID:      primitive.NewObjectID(),
		OwnerID: ownerID,
		Token:   util.MakeToken(keyLen),
		Secret:  util.MakeToken(keyLen),
		Valid:   true,
	}
	res, err := k.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}
	doc.ID = res.InsertedID.(primitive.ObjectID)
	return doc, nil
}

func (k *Keys) Get(ctx context.Context, token string) (*Key, error) {
	var doc *Key
	res := k.col.FindOne(ctx, bson.M{"token": token})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (k *Keys) List(ctx context.Context, ownerID primitive.ObjectID) ([]Key, error) {
	cursor, err := k.col.Find(ctx, bson.M{"owner_id": ownerID})
	if err != nil {
		return nil, err
	}
	var docs []Key
	for cursor.Next(ctx) {
		var doc Key
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return docs, nil
}

func (k *Keys) Invalidate(ctx context.Context, token string) error {
	res, err := k.col.UpdateOne(ctx, bson.M{"token": token}, bson.M{"$set": bson.M{"valid": false}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
