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
	inviteDur = time.Hour * 24 * 7 * 30
)

type Invite struct {
	Token     string
	Org       string
	From      crypto.PubKey
	EmailTo   string
	ExpiresAt time.Time
}

type Invites struct {
	col *mongo.Collection
}

func NewInvites(ctx context.Context, db *mongo.Database) (*Invites, error) {
	i := &Invites{col: db.Collection("invites")}
	_, err := i.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{"org", 1}},
		},
		{
			Keys: bson.D{{"from_id", 1}},
		},
	})
	return i, err
}

func (i *Invites) Create(ctx context.Context, from crypto.PubKey, org, emailTo string) (*Invite, error) {
	doc := &Invite{
		Token:     util.MakeToken(tokenLen),
		Org:       org,
		From:      from,
		EmailTo:   emailTo,
		ExpiresAt: time.Now().Add(inviteDur),
	}
	fromID, err := crypto.MarshalPublicKey(from)
	if err != nil {
		return nil, err
	}
	if _, err := i.col.InsertOne(ctx, bson.M{
		"_id":        doc.Token,
		"org":        doc.Org,
		"from_id":    fromID,
		"email_to":   doc.EmailTo,
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

func decodeInvite(raw bson.M) (*Invite, error) {
	from, err := crypto.UnmarshalPublicKey(raw["from_id"].(primitive.Binary).Data)
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
		ExpiresAt: expiry,
	}, nil
}
