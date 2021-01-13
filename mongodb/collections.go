package mongodb

import (
	"context"
	"time"

	"github.com/textileio/textile/v2/mongodb/migrations"
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

	Threads         *Threads
	APIKeys         *APIKeys
	IPNSKeys        *IPNSKeys
	BucketArchives  *BucketArchives
	ArchiveTracking *ArchiveTracking
}

// NewCollections gets or create store instances for active collections.
func NewCollections(ctx context.Context, uri, database string, hub bool) (*Collections, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	db := client.Database(database)
	if err = migrations.Migrate(db); err != nil {
		return nil, err
	}
	c := &Collections{m: client}
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
	}
	c.ArchiveTracking, err = NewArchiveTracking(ctx, db)
	if err != nil {
		return nil, err
	}

	c.IPNSKeys, err = NewIPNSKeys(ctx, db)
	if err != nil {
		return nil, err
	}
	c.BucketArchives, err = NewBucketArchives(ctx, db)
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
