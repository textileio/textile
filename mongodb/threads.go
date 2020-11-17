package mongodb

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/v2/api/common"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	threadNameRx *regexp.Regexp

	ErrInvalidThreadName = fmt.Errorf("name may only contain alphanumeric characters or non-consecutive hyphens, and cannot begin or end with a hyphen")
)

func init() {
	threadNameRx = regexp.MustCompile(`^[A-Za-z0-9]+(?:[-][A-Za-z0-9]+)*$`)
}

type Thread struct {
	ID        thread.ID
	Owner     thread.PubKey
	Name      string
	Key       string
	IsDB      bool
	CreatedAt time.Time
}

type Threads struct {
	col *mongo.Collection
}

func NewThreads(ctx context.Context, db *mongo.Database) (*Threads, error) {
	t := &Threads{col: db.Collection("threads")}
	_, err := t.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{primitive.E{Key: "_id.owner", Value: 1}, primitive.E{Key: "name", Value: 1}},
			Options: options.Index().SetUnique(true).
				SetPartialFilterExpression(bson.D{primitive.E{Key: "name", Value: bson.M{"$exists": 1}}}).
				SetCollation(&options.Collation{Locale: "en", Strength: 2}),
		},
		{
			Keys: bson.D{primitive.E{Key: "_id.thread", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "key_id", Value: 1}},
		},
	})
	return t, err
}

func (t *Threads) Create(ctx context.Context, id thread.ID, owner thread.PubKey, isDB bool) (*Thread, error) {
	name, _ := common.ThreadNameFromContext(ctx)
	if name != "" && !threadNameRx.MatchString(name) {
		return nil, ErrInvalidThreadName
	}
	key, _ := common.APIKeyFromContext(ctx)
	doc := &Thread{
		ID:        id,
		Owner:     owner,
		Name:      name,
		Key:       key,
		IsDB:      isDB,
		CreatedAt: time.Now(),
	}
	ownerID, err := owner.MarshalBinary()
	if err != nil {
		return nil, err
	}
	raw := bson.M{
		"_id":        bson.D{primitive.E{Key: "owner", Value: ownerID}, primitive.E{Key: "thread", Value: id.Bytes()}},
		"key_id":     doc.Key,
		"is_db":      doc.IsDB,
		"created_at": doc.CreatedAt,
	}
	if doc.Name != "" {
		raw["name"] = doc.Name
	}
	if _, err := t.col.InsertOne(ctx, raw); err != nil {
		return nil, err
	}
	return doc, nil
}

func (t *Threads) Get(ctx context.Context, id thread.ID, owner thread.PubKey) (*Thread, error) {
	ownerID, err := owner.MarshalBinary()
	if err != nil {
		return nil, err
	}
	res := t.col.FindOne(ctx, bson.M{"_id": bson.D{primitive.E{Key: "owner", Value: ownerID}, primitive.E{Key: "thread", Value: id.Bytes()}}})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeThread(raw)
}

func (t *Threads) GetByName(ctx context.Context, name string, owner thread.PubKey) (*Thread, error) {
	ownerID, err := owner.MarshalBinary()
	if err != nil {
		return nil, err
	}
	res := t.col.FindOne(ctx, bson.M{"_id.owner": ownerID, "name": name})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeThread(raw)
}

func (t *Threads) ListByOwner(ctx context.Context, owner thread.PubKey) ([]Thread, error) {
	ownerID, err := owner.MarshalBinary()
	if err != nil {
		return nil, err
	}
	cursor, err := t.col.Find(ctx, bson.M{"_id.owner": ownerID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []Thread
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}
		doc, err := decodeThread(raw)
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

func (t *Threads) ListByKey(ctx context.Context, key string) ([]Thread, error) {
	cursor, err := t.col.Find(ctx, bson.M{"key_id": key})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []Thread
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}
		doc, err := decodeThread(raw)
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

func (t *Threads) Delete(ctx context.Context, id thread.ID, owner thread.PubKey) error {
	ownerID, err := owner.MarshalBinary()
	if err != nil {
		return err
	}
	res, err := t.col.DeleteOne(ctx, bson.M{"_id": bson.D{primitive.E{Key: "owner", Value: ownerID}, primitive.E{Key: "thread", Value: id.Bytes()}}})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (t *Threads) DeleteByOwner(ctx context.Context, owner thread.PubKey) error {
	ownerID, err := owner.MarshalBinary()
	if err != nil {
		return err
	}
	_, err = t.col.DeleteMany(ctx, bson.M{"_id.owner": ownerID})
	return err
}

func decodeThread(raw bson.M) (*Thread, error) {
	rid := raw["_id"].(bson.M)
	owner := &thread.Libp2pPubKey{}
	err := owner.UnmarshalBinary(rid["owner"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	id, err := thread.Cast(rid["thread"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var name string
	if raw["name"] != nil {
		name = raw["name"].(string)
	}
	var key string
	if raw["key_id"] != nil {
		key = raw["key_id"].(string)
	}
	var isDB bool
	if v, ok := raw["is_db"]; ok {
		isDB = v.(bool)
	} else {
		isDB = true
	}
	var created time.Time
	if v, ok := raw["created_at"]; ok {
		created = v.(primitive.DateTime).Time()
	}
	return &Thread{
		ID:        id,
		Owner:     owner,
		Name:      name,
		Key:       key,
		IsDB:      isDB,
		CreatedAt: created,
	}, nil
}
