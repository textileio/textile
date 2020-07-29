package mongodb

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	tokenLen = 44

	DuplicateErrMsg = "E11000 duplicate key error"
)

type ctxKey string

type Collections struct {
	m *mongo.Client

	Sessions *Sessions
	Accounts *Accounts
	Invites  *Invites

	Threads      *Threads
	APIKeys      *APIKeys
	IPNSKeys     *IPNSKeys
	FFSInstances *FFSInstances

	Users *Users
}

// NewCollections gets or create store instances for active collections.
func NewCollections(ctx context.Context, uri, dbName string, hub bool) (*Collections, error) {
	m, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	db := m.Database(dbName)
	c := &Collections{m: m}

	if hub {
		c.Sessions, err = NewSessions(ctx, db)
		if err != nil {
			return nil, err
		}
		c.Accounts, err = NewAccounts(ctx, db)
		if err != nil {
			return nil, err
		}
		c.Invites, err = NewInvites(ctx, db)
		if err != nil {
			return nil, err
		}
		c.Threads, err = NewThreads(ctx, db)
		if err != nil {
			return nil, err
		}
		c.APIKeys, err = NewAPIKeys(ctx, db)
		if err != nil {
			return nil, err
		}
		c.Users, err = NewUsers(ctx, db)
		if err != nil {
			return nil, err
		}
	}
	c.IPNSKeys, err = NewIPNSKeys(ctx, db)
	if err != nil {
		return nil, err
	}
	c.FFSInstances, err = NewFFSInstances(ctx, db)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Collections) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return c.m.Disconnect(ctx)
}
