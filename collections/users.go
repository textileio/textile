package collections

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type User struct {
	ID        primitive.ObjectID `bson:"_id"`
	DeviceID  string             `bson:"device_id"`
	CreatedAt time.Time          `bson:"created_at"`
}

type Users struct {
	col *mongo.Collection
}

func NewUsers(ctx context.Context, db *mongo.Database) (*Users, error) {
	u := &Users{col: db.Collection("users")}
	_, err := u.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"device_id", 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	return u, err
}

func (u *Users) GetOrCreate(ctx context.Context, deviceID string) (user *User, err error) {
	doc := &User{
		ID:        primitive.NewObjectID(),
		DeviceID:  deviceID,
		CreatedAt: time.Now(),
	}
	if _, err := u.col.InsertOne(ctx, doc); err != nil {
		if _, ok := err.(mongo.WriteException); ok {
			return u.Get(ctx, deviceID)
		}
		return nil, err
	}
	return doc, nil
}

func (u *Users) Get(ctx context.Context, deviceID string) (*User, error) {
	var doc *User
	res := u.col.FindOne(ctx, bson.M{"device_id": deviceID})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (u *Users) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := u.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
