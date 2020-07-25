package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type User struct {
	Key              crypto.PubKey
	BucketsTotalSize int64
	CreatedAt        time.Time
}

func NewUserContext(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, ctxKey("user"), user)
}

func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(ctxKey("user")).(*User)
	return user, ok
}

type Users struct {
	col *mongo.Collection
}

func NewUsers(_ context.Context, db *mongo.Database) (*Users, error) {
	return &Users{col: db.Collection("users")}, nil
}

func (u *Users) Create(ctx context.Context, key crypto.PubKey) error {
	doc := &User{
		Key:       key,
		CreatedAt: time.Now(),
	}
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return err
	}
	if _, err := u.col.InsertOne(ctx, bson.M{
		"_id":                id,
		"buckets_total_size": int64(0),
		"created_at":         doc.CreatedAt,
	}); err != nil {
		if _, ok := err.(mongo.WriteException); ok {
			return nil
		}
		return err
	}
	return nil
}

func (u *Users) Get(ctx context.Context, key crypto.PubKey) (*User, error) {
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return nil, err
	}
	var raw bson.M
	res := u.col.FindOne(ctx, bson.M{"_id": id})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeUser(raw)
}

func (u *Users) Delete(ctx context.Context, key crypto.PubKey) error {
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return err
	}
	res, err := u.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (u *Users) SetBucketsTotalSize(ctx context.Context, key crypto.PubKey, newTotalSize int64) error {
	if newTotalSize < 0 {
		return fmt.Errorf("new size %d must be positive", newTotalSize)
	}
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return err
	}
	res, err := u.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"buckets_total_size": newTotalSize}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func decodeUser(raw bson.M) (*User, error) {
	key, err := crypto.UnmarshalPublicKey(raw["_id"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var created time.Time
	if v, ok := raw["created_at"]; ok {
		created = v.(primitive.DateTime).Time()
	}
	var bucketsTotalSize int64
	if v, ok := raw["buckets_total_size"]; ok {
		bucketsTotalSize = v.(int64)
	}
	return &User{
		Key:              key,
		BucketsTotalSize: bucketsTotalSize,
		CreatedAt:        created,
	}, nil
}
