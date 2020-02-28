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

type Team struct {
	ID        primitive.ObjectID   `bson:"_id"`
	OwnerID   primitive.ObjectID   `bson:"owner_id"`
	Name      string               `bson:"name"`
	Members   []primitive.ObjectID `bson:"members"`
	CreatedAt time.Time            `bson:"created_at"`
}

type Teams struct {
	col *mongo.Collection
}

func NewTeams(ctx context.Context, db *mongo.Database) (*Teams, error) {
	t := &Teams{col: db.Collection("teams")}
	_, err := t.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{"owner_id", 1}, {"name", 1}},
		},
		{
			Keys:    bson.D{{"members", 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	return t, err
}

func (t *Teams) Create(ctx context.Context, ownerID primitive.ObjectID, name string) (*Team, error) {
	validName, err := toValidName(name)
	if err != nil {
		return nil, err
	}
	doc := &Team{
		OwnerID:   ownerID,
		Name:      validName,
		Members:   []primitive.ObjectID{},
		CreatedAt: time.Now(),
	}
	res, err := t.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}
	doc.ID = res.InsertedID.(primitive.ObjectID)
	return doc, nil
}

func (t *Teams) Get(ctx context.Context, id primitive.ObjectID) (*Team, error) {
	var doc *Team
	res := t.col.FindOne(ctx, bson.M{"_id": id})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (t *Teams) List(ctx context.Context, memberID primitive.ObjectID) ([]Team, error) {
	filter := bson.M{"members": bson.M{"$elemMatch": bson.M{"$eq": memberID}}}
	cursor, err := t.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var docs []Team
	for cursor.Next(ctx) {
		var doc Team
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

func (t *Teams) HasMember(ctx context.Context, id, memberID primitive.ObjectID) (bool, error) {
	filter := bson.M{"_id": id, "members": bson.M{"$elemMatch": bson.M{"$eq": memberID}}}
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

func (t *Teams) AddMember(ctx context.Context, id, memberID primitive.ObjectID) error {
	res, err := t.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$push": bson.M{"members": memberID}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (t *Teams) RemoveMember(ctx context.Context, id, memberID primitive.ObjectID) error {
	res, err := t.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$pull": bson.M{"members": memberID}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (t *Teams) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := t.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
