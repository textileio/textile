package collections

import (
	"context"
	"time"

	"github.com/gosimple/slug"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	dbName   = "textile"
	tokenLen = 44
)

func init() {
	slug.MaxLength = 64
}

type ctxKey string

type Collections struct {
	m *mongo.Client

	Sessions   *Sessions
	Developers *Developers
	Orgs       *Orgs
	Invites    *Invites

	Threads *Threads

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
	databases, err := NewThreads(ctx, db)
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

		Threads: databases,

		Users: users,
	}, nil
}

func (c *Collections) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return c.m.Disconnect(ctx)
}
