package collections

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Org struct {
	ID        primitive.ObjectID   `bson:"_id"`
	OwnerID   primitive.ObjectID   `bson:"owner_id"`
	Name      string               `bson:"name"`
	StoreID   string               `bson:"store_id"`
	MemberIDs []primitive.ObjectID `bson:"member_ids"`
	CreatedAt time.Time            `bson:"created_at"`
}

type Orgs struct {
	col *mongo.Collection
}

func NewOrgs(ctx context.Context, db *mongo.Database) (*Orgs, error) {
	t := &Orgs{col: db.Collection("orgs")}
	_, err := t.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"owner_id", 1}, {"name", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{"member_ids", 1}},
		},
	})
	return t, err
}

func (t *Orgs) Create(ctx context.Context, ownerID primitive.ObjectID, name, storeID string) (*Org, error) {
	validName, err := toValidName(name)
	if err != nil {
		return nil, err
	}
	doc := &Org{
		ID:        primitive.NewObjectID(),
		OwnerID:   ownerID,
		Name:      validName,
		StoreID:   storeID,
		MemberIDs: []primitive.ObjectID{},
		CreatedAt: time.Now(),
	}
	res, err := t.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}
	doc.ID = res.InsertedID.(primitive.ObjectID)
	return doc, nil
}

func (t *Orgs) Get(ctx context.Context, id primitive.ObjectID) (*Org, error) {
	var doc *Org
	res := t.col.FindOne(ctx, bson.M{"_id": id})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (t *Orgs) List(ctx context.Context, memberID primitive.ObjectID) ([]Org, error) {
	filter := bson.M{"member_ids": bson.M{"$elemMatch": bson.M{"$eq": memberID}}}
	cursor, err := t.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var docs []Org
	for cursor.Next(ctx) {
		var doc Org
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

func (t *Orgs) HasMember(ctx context.Context, id, memberID primitive.ObjectID) (bool, error) {
	filter := bson.M{"_id": id, "member_ids": bson.M{"$elemMatch": bson.M{"$eq": memberID}}}
	res := t.col.FindOne(ctx, filter)
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return false, nil
		} else {
			return false, res.Err()
		}
	}
	return true, nil
}

func (t *Orgs) AddMember(ctx context.Context, id, memberID primitive.ObjectID) error {
	res, err := t.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$addToSet": bson.M{"member_ids": memberID}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (t *Orgs) RemoveMember(ctx context.Context, id, memberID primitive.ObjectID) error {
	res, err := t.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$pull": bson.M{"member_ids": memberID}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (t *Orgs) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := t.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
