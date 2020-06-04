package collections

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	domainRx *regexp.Regexp

	ErrInvalidDomain = fmt.Errorf("invalid domain name")
)

const (
	keyLen    = 16
	secretLen = 24
)

func init() {
	// Copied from https://regex101.com/library/SEg6KL
	domainRx = regexp.MustCompile(`(?i)^(?:[_a-z0-9](?:[_a-z0-9-]{0,61}[a-z0-9]\.)|(?:[0-9]+/[0-9]{2})\.)+(?:[a-z](?:[a-z0-9-]{0,61}[a-z0-9])?)?$`)
}

type APIKeyType int

const (
	AccountKey APIKeyType = iota
	UserKey
)

type APIKey struct {
	Key       string
	Secret    string
	Owner     crypto.PubKey
	Type      APIKeyType
	Domains   []string
	Valid     bool
	CreatedAt time.Time
}

func (k *APIKey) AllowsOrigin(origin string) bool {
	parts := strings.SplitN(origin, "://", 2)
	if len(parts) < 2 {
		return false
	}
	domain := parts[1]
	for _, d := range k.Domains {
		if d == domain {
			return true
		}
	}
	return false
}

func NewAPIKeyContext(ctx context.Context, key *APIKey) context.Context {
	return context.WithValue(ctx, ctxKey("apiKey"), key)
}

func APIKeyFromContext(ctx context.Context) (*APIKey, bool) {
	key, ok := ctx.Value(ctxKey("apiKey")).(*APIKey)
	return key, ok
}

type APIKeys struct {
	col *mongo.Collection
}

func NewAPIKeys(ctx context.Context, db *mongo.Database) (*APIKeys, error) {
	k := &APIKeys{col: db.Collection("apikeys")}
	_, err := k.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{"owner_id", 1}},
		},
	})
	return k, err
}

func (k *APIKeys) Create(ctx context.Context, owner crypto.PubKey, keyType APIKeyType, domains []string) (*APIKey, error) {
	for i, d := range domains {
		d = strings.TrimSpace(d)
		if err := validateDomain(d); err != nil {
			return nil, err
		}
		domains[i] = d
	}
	doc := &APIKey{
		Key:       util.MakeToken(keyLen),
		Secret:    util.MakeToken(secretLen),
		Owner:     owner,
		Type:      keyType,
		Domains:   domains,
		Valid:     true,
		CreatedAt: time.Now(),
	}
	ownerID, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return nil, err
	}
	if _, err := k.col.InsertOne(ctx, bson.M{
		"_id":        doc.Key,
		"secret":     doc.Secret,
		"owner_id":   ownerID,
		"type":       int32(doc.Type),
		"domains":    doc.Domains,
		"valid":      doc.Valid,
		"created_at": doc.CreatedAt,
	}); err != nil {
		return nil, err
	}
	return doc, nil
}

func (k *APIKeys) Get(ctx context.Context, key string) (*APIKey, error) {
	res := k.col.FindOne(ctx, bson.M{"_id": key})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeAPIKey(raw)
}

func (k *APIKeys) ListByOwner(ctx context.Context, owner crypto.PubKey) ([]APIKey, error) {
	ownerID, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return nil, err
	}
	cursor, err := k.col.Find(ctx, bson.M{"owner_id": ownerID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []APIKey
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}
		doc, err := decodeAPIKey(raw)
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

func (k *APIKeys) Invalidate(ctx context.Context, key string) error {
	res, err := k.col.UpdateOne(ctx, bson.M{"_id": key}, bson.M{"$set": bson.M{"valid": false}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (k *APIKeys) DeleteByOwner(ctx context.Context, owner crypto.PubKey) error {
	ownerID, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return err
	}
	_, err = k.col.DeleteMany(ctx, bson.M{"owner_id": ownerID})
	return err
}

func validateDomain(domain string) error {
	if !domainRx.MatchString(domain) {
		return ErrInvalidDomain
	}
	return nil
}

func decodeAPIKey(raw bson.M) (*APIKey, error) {
	owner, err := crypto.UnmarshalPublicKey(raw["owner_id"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var created time.Time
	if v, ok := raw["created_at"]; ok {
		created = v.(primitive.DateTime).Time()
	}
	var domains []string
	if v, ok := raw["domains"]; ok && v != nil {
		list := v.(primitive.A)
		for _, d := range list {
			domains = append(domains, d.(string))
		}
	}
	return &APIKey{
		Key:       raw["_id"].(string),
		Secret:    raw["secret"].(string),
		Owner:     owner,
		Type:      APIKeyType(raw["type"].(int32)),
		Domains:   domains,
		Valid:     raw["valid"].(bool),
		CreatedAt: created,
	}, nil
}
