package mongodb

import (
	"context"
	"time"

	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/v2/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	inviteDur = time.Hour * 24 * 7 * 30
)

type Invite struct {
	Token     string
	Org       string
	From      thread.PubKey
	EmailTo   string
	Accepted  bool
	ExpiresAt time.Time
}

type Invites struct {
	col *mongo.Collection
}

func NewInvites(ctx context.Context, db *mongo.Database) (*Invites, error) {
	i := &Invites{col: db.Collection("invites")}
	_, err := i.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{primitive.E{Key: "org", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "from_id", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "email_to", Value: 1}},
		},
	})
	return i, err
}

func (i *Invites) Create(ctx context.Context, from thread.PubKey, org, emailTo string) (*Invite, error) {
	doc := &Invite{
		Token:     util.MakeToken(tokenLen),
		Org:       org,
		From:      from,
		EmailTo:   emailTo,
		Accepted:  false,
		ExpiresAt: time.Now().Add(inviteDur),
	}
	fromID, err := from.MarshalBinary()
	if err != nil {
		return nil, err
	}
	if _, err := i.col.InsertOne(ctx, bson.M{
		"_id":        doc.Token,
		"org":        doc.Org,
		"from_id":    fromID,
		"email_to":   doc.EmailTo,
		"accepted":   doc.Accepted,
		"expires_at": doc.ExpiresAt,
	}); err != nil {
		return nil, err
	}
	return doc, nil
}

func (i *Invites) Get(ctx context.Context, token string) (*Invite, error) {
	res := i.col.FindOne(ctx, bson.M{"_id": token})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeInvite(raw)
}

func (i *Invites) ListByEmail(ctx context.Context, email string) ([]Invite, error) {
	cursor, err := i.col.Find(ctx, bson.M{"email_to": email})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []Invite
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}
		doc, err := decodeInvite(raw)
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

func (i *Invites) Accept(ctx context.Context, token string) error {
	res, err := i.col.UpdateOne(ctx, bson.M{"_id": token}, bson.M{"$set": bson.M{"accepted": true}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (i *Invites) Delete(ctx context.Context, token string) error {
	res, err := i.col.DeleteOne(ctx, bson.M{"_id": token})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (i *Invites) DeleteByFrom(ctx context.Context, from thread.PubKey) error {
	fromID, err := from.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = i.col.DeleteMany(ctx, bson.M{"from_id": fromID})
	return err
}

func (i *Invites) DeleteByOrg(ctx context.Context, org string) error {
	_, err := i.col.DeleteMany(ctx, bson.M{"org": org})
	return err
}

func (i *Invites) DeleteByFromAndOrg(ctx context.Context, from thread.PubKey, org string) error {
	fromID, err := from.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = i.col.DeleteMany(ctx, bson.M{"from_id": fromID, "org": org})
	return err
}

func decodeInvite(raw bson.M) (*Invite, error) {
	from := &thread.Libp2pPubKey{}
	err := from.UnmarshalBinary(raw["from_id"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var expiry time.Time
	if v, ok := raw["expires_at"]; ok {
		expiry = v.(primitive.DateTime).Time()
	}
	return &Invite{
		Token:     raw["_id"].(string),
		Org:       raw["org"].(string),
		From:      from,
		EmailTo:   raw["email_to"].(string),
		Accepted:  raw["accepted"].(bool),
		ExpiresAt: expiry,
	}, nil
}
