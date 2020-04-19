package collections

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	usernameRx *regexp.Regexp

	ErrInvalidUsername = fmt.Errorf("username may only contain alphanumeric characters or single hyphens, and cannot begin or end with a hyphen")
)

func init() {
	usernameRx = regexp.MustCompile(`^[a-zA-Z0-9]+(?:-[a-zA-Z0-9]+)?$`)
}

type Developer struct {
	Key       crypto.PubKey
	Secret    crypto.PrivKey
	Email     string
	Username  string
	Token     thread.Token
	CreatedAt time.Time
}

func NewDevContext(ctx context.Context, dev *Developer) context.Context {
	return context.WithValue(ctx, ctxKey("developer"), dev)
}

func DevFromContext(ctx context.Context) (*Developer, bool) {
	dev, ok := ctx.Value(ctxKey("developer")).(*Developer)
	return dev, ok
}

type Developers struct {
	col *mongo.Collection
}

func NewDevelopers(ctx context.Context, db *mongo.Database) (*Developers, error) {
	d := &Developers{col: db.Collection("developers")}
	_, err := d.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"email", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{"username", 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	return d, err
}

func (d *Developers) Create(ctx context.Context, username, email string) (*Developer, error) {
	if err := d.ValidateUsername(username); err != nil {
		return nil, err
	}
	skey, key, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	doc := &Developer{
		Key:       key,
		Secret:    skey,
		Email:     email,
		Username:  username,
		CreatedAt: time.Now(),
	}
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return nil, err
	}
	secret, err := crypto.MarshalPrivateKey(skey)
	if err != nil {
		return nil, err
	}
	if _, err := d.col.InsertOne(ctx, bson.M{
		"_id":        id,
		"secret":     secret,
		"email":      doc.Email,
		"username":   doc.Username,
		"created_at": doc.CreatedAt,
	}); err != nil {
		return nil, err
	}
	return doc, nil
}

func (d *Developers) Get(ctx context.Context, key crypto.PubKey) (*Developer, error) {
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return nil, err
	}
	res := d.col.FindOne(ctx, bson.M{"_id": id})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeDeveloper(raw)
}

func (d *Developers) GetByUsernameOrEmail(ctx context.Context, usernameOrEmail string) (*Developer, error) {
	res := d.col.FindOne(ctx, bson.D{{"$or", bson.A{bson.D{{"email", usernameOrEmail}}, bson.D{{"username", usernameOrEmail}}}}})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeDeveloper(raw)
}

func (d *Developers) ValidateUsername(username string) error {
	if !usernameRx.MatchString(username) {
		return ErrInvalidUsername
	}
	return nil
}

func (d *Developers) IsUsernameAvailable(ctx context.Context, username string) error {
	if err := d.ValidateUsername(username); err != nil {
		return err
	}
	res := d.col.FindOne(ctx, bson.M{"username": username})
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return nil
		}
		return res.Err()
	}
	return fmt.Errorf("username '%s' is not available", username)
}

func (d *Developers) SetToken(ctx context.Context, key crypto.PubKey, token thread.Token) error {
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return err
	}
	res, err := d.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"token": token}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (d *Developers) ListMembers(ctx context.Context, members []Member) ([]Developer, error) {
	keys := make([][]byte, len(members))
	var err error
	for i, m := range members {
		keys[i], err = crypto.MarshalPublicKey(m.Key)
		if err != nil {
			return nil, err
		}
	}
	cursor, err := d.col.Find(ctx, bson.M{"_id": bson.M{"$in": keys}})
	if err != nil {
		return nil, err
	}
	var docs []Developer
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}
		doc, err := decodeDeveloper(raw)
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

// @todo: Developer must first delete bucket and orgs they own
// @todo: Delete associated sessions, store, remove from orgs
func (d *Developers) Delete(ctx context.Context, key crypto.PubKey) error {
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return err
	}
	res, err := d.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func decodeDeveloper(raw bson.M) (*Developer, error) {
	skey, err := crypto.UnmarshalPrivateKey(raw["secret"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var token thread.Token
	if v, ok := raw["token"]; ok {
		token = thread.Token(v.(string))
	}
	var created time.Time
	if v, ok := raw["created_at"]; ok {
		created = v.(primitive.DateTime).Time()
	}
	return &Developer{
		Key:       skey.GetPublic(),
		Secret:    skey,
		Email:     raw["email"].(string),
		Username:  raw["username"].(string),
		Token:     token,
		CreatedAt: created,
	}, nil
}
