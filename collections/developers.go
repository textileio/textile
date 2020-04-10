package collections

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Developer struct {
	ID             primitive.ObjectID `bson:"_id"`
	Email          string             `bson:"email"`
	Username       string             `bson:"username"`
	ThreadIdentity crypto.PrivKey     `bson:"thread_identity"`
	ThreadToken    thread.Token       `bson:"thread_token"`
	CreatedAt      time.Time          `bson:"created_at"`
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
		{
			Keys:    bson.D{{"thread_identity", 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
	})
	return d, err
}

func (d *Developers) GetOrCreate(ctx context.Context, username, email string) (*Developer, error) {
	validUsername, err := util.ToValidName(username)
	if err != nil {
		return nil, err
	}
	sk, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	doc := &Developer{
		ID:             primitive.NewObjectID(),
		Email:          email,
		Username:       validUsername,
		ThreadIdentity: sk,
		CreatedAt:      time.Now(),
	}
	idb, err := crypto.MarshalPrivateKey(doc.ThreadIdentity)
	if err != nil {
		return nil, err
	}
	if _, err := d.col.InsertOne(ctx, bson.M{
		"_id":             doc.ID,
		"email":           doc.Email,
		"username":        doc.Username,
		"thread_identity": idb,
		"created_at":      doc.CreatedAt,
	}); err != nil {
		if _, ok := err.(mongo.WriteException); ok {
			res := d.col.FindOne(ctx, bson.M{"email": email})
			if res.Err() != nil {
				return nil, res.Err()
			}
			var raw bson.M
			if err := res.Decode(&raw); err != nil {
				return nil, err
			}
			return decodeDeveloper(raw)
		}
		return nil, err
	}
	return doc, nil
}

func (d *Developers) Get(ctx context.Context, id primitive.ObjectID) (*Developer, error) {
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

func (d *Developers) SetDBToken(ctx context.Context, id primitive.ObjectID, token thread.Token) error {
	res, err := d.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"thread_token": token}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (d *Developers) ListMembers(ctx context.Context, members []Member) ([]Developer, error) {
	ids := make([]primitive.ObjectID, len(members))
	for i, m := range members {
		ids[i] = m.ID
	}
	cursor, err := d.col.Find(ctx, bson.M{"_id": bson.M{"$in": ids}})
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
func (d *Developers) Delete(ctx context.Context, id primitive.ObjectID) error {
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
	id, err := crypto.UnmarshalPrivateKey(raw["thread_identity"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var token thread.Token
	if v, ok := raw["thread_token"]; ok {
		token = thread.Token(v.(string))
	}
	var created time.Time
	if v, ok := raw["created_at"]; ok {
		created = v.(primitive.DateTime).Time()
	}
	return &Developer{
		ID:             raw["_id"].(primitive.ObjectID),
		Email:          raw["email"].(string),
		Username:       raw["username"].(string),
		ThreadIdentity: id,
		ThreadToken:    token,
		CreatedAt:      created,
	}, nil
}
