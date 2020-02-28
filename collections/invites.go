package collections

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	inviteDur = time.Hour * 24 * 7 * 30
)

type Invite struct {
	ID        primitive.ObjectID `bson:"_id"`
	TeamID    primitive.ObjectID `bson:"team_id"`
	FromID    primitive.ObjectID `bson:"from_id"`
	EmailTo   string             `bson:"email_to"`
	ExpiresAt time.Time          `bson:"expires_at"`
}

type Invites struct {
	col *mongo.Collection
}

func NewInvites(ctx context.Context, db *mongo.Database) (*Invites, error) {
	i := &Invites{col: db.Collection("invites")}
	_, err := i.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{"team_id", 1}},
		},
		{
			Keys: bson.D{{"from_id", 1}},
		},
	})
	return i, err
}

func (i *Invites) Create(ctx context.Context, teamID, fromID primitive.ObjectID, emailTo string) (*Invite, error) {
	doc := &Invite{
		TeamID:    teamID,
		FromID:    fromID,
		EmailTo:   emailTo,
		ExpiresAt: time.Now().Add(inviteDur),
	}
	res, err := i.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}
	doc.ID = res.InsertedID.(primitive.ObjectID)
	return doc, nil
}

func (i *Invites) Get(ctx context.Context, id primitive.ObjectID) (*Invite, error) {
	var doc *Invite
	res := i.col.FindOne(ctx, bson.M{"_id": id})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (i *Invites) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := i.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
