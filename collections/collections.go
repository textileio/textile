package collections

import (
	"context"
	"time"

	"github.com/gosimple/slug"
	logging "github.com/ipfs/go-log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	dbName = "textile"
)

var (
	_ = logging.Logger("collections")
)

func init() {
	slug.MaxLength = 64
}

type key int

type Collections struct {
	m *mongo.Client

	Sessions   *Sessions
	Developers *Developers
	Orgs       *Orgs
	Invites    *Invites

	Buckets      *Buckets
	BucketTokens *BucketTokens

	Users *Users
}

// NewCollections gets or create store instances for active collections.
func NewCollections(ctx context.Context, uri string) (*Collections, error) {
	m, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	db := m.Database(dbName)

	sessions, err := NewSessions(ctx, db)
	if err != nil {
		return nil, err
	}
	developers, err := NewDevelopers(ctx, db)
	if err != nil {
		return nil, err
	}
	teams, err := NewOrgs(ctx, db)
	if err != nil {
		return nil, err
	}
	invites, err := NewInvites(ctx, db)
	if err != nil {
		return nil, err
	}
	buckets, err := NewBuckets(ctx, db)
	if err != nil {
		return nil, err
	}
	bucketTokens, err := NewBucketTokens(ctx, db)
	if err != nil {
		return nil, err
	}
	users, err := NewUsers(ctx, db)
	if err != nil {
		return nil, err
	}

	return &Collections{
		m: m,

		Sessions:   sessions,
		Developers: developers,
		Orgs:       teams,
		Invites:    invites,

		Buckets:      buckets,
		BucketTokens: bucketTokens,

		Users: users,
	}, nil
}

func (c *Collections) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return c.m.Disconnect(ctx)
}

// @todo: Move the auth handling

type authKey string

func AuthCtx(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, authKey("token"), token)
}

type TokenAuth struct{}

func (t TokenAuth) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	md := map[string]string{}
	token, ok := ctx.Value(authKey("token")).(string)
	if ok && token != "" {
		md["authorization"] = "bearer " + token
	}
	return md, nil
}

func (t TokenAuth) RequireTransportSecurity() bool {
	return false
}
