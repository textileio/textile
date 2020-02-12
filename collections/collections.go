package collections

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/alecthomas/jsonschema"
	"github.com/google/uuid"
	"github.com/ipfs/go-datastore"
	logging "github.com/ipfs/go-log"
	"github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/store"
)

var (
	log = logging.Logger("collections")

	dsDevelopersKey = datastore.NewKey("/developers")
	dsSessionsKey   = datastore.NewKey("/sessions")
	dsTeamsKey      = datastore.NewKey("/teams")
	dsInvitesKey    = datastore.NewKey("/invites")
	dsProjectsKey   = datastore.NewKey("/projects")

	dsTokensKey = datastore.NewKey("/tokens")
	dsUsersKey  = datastore.NewKey("/users")

	dsBucketsKey = datastore.NewKey("/buckets")
)

type Collection interface {
	GetName() string
	GetInstance() interface{}
	GetIndexes() []*store.IndexConfig
	GetStoreID() *uuid.UUID
}

type Collections struct {
	threads *client.Client
	token   string
	ds      datastore.Datastore

	Developers *Developers
	Sessions   *Sessions
	Teams      *Teams
	Invites    *Invites
	Projects   *Projects

	Tokens *Tokens
	Users  *Users

	Buckets *Buckets
}

// NewCollections gets or create store instances for active collections.
func NewCollections(
	ctx context.Context,
	threads *client.Client,
	token string,
	ds datastore.Datastore,
) (c *Collections, err error) {
	c = &Collections{
		threads: threads,
		token:   token,
		ds:      ds,

		Developers: &Developers{threads: threads, token: token},
		Sessions:   &Sessions{threads: threads, token: token},
		Teams:      &Teams{threads: threads, token: token},
		Invites:    &Invites{threads: threads, token: token},
		Projects:   &Projects{threads: threads, token: token},

		Tokens: &Tokens{threads: threads, token: token},
		Users:  &Users{threads: threads, token: token},

		Buckets: &Buckets{threads: threads, token: token},
	}
	ctx = AuthCtx(ctx, c.token)

	c.Developers.storeID, err = c.addCollection(ctx, c.Developers, dsDevelopersKey)
	if err != nil {
		return nil, err
	}
	c.Sessions.storeID, err = c.addCollection(ctx, c.Sessions, dsSessionsKey)
	if err != nil {
		return nil, err
	}
	c.Teams.storeID, err = c.addCollection(ctx, c.Teams, dsTeamsKey)
	if err != nil {
		return nil, err
	}
	c.Invites.storeID, err = c.addCollection(ctx, c.Invites, dsInvitesKey)
	if err != nil {
		return nil, err
	}
	c.Projects.storeID, err = c.addCollection(ctx, c.Projects, dsProjectsKey)
	if err != nil {
		return nil, err
	}
	c.Tokens.storeID, err = c.addCollection(ctx, c.Tokens, dsTokensKey)
	if err != nil {
		return nil, err
	}
	c.Users.storeID, err = c.addCollection(ctx, c.Users, dsUsersKey)
	if err != nil {
		return nil, err
	}
	c.Buckets.storeID, err = c.addCollection(ctx, c.Buckets, dsBucketsKey)
	if err != nil {
		return nil, err
	}

	log.Debugf("developers store: %s", c.Developers.GetStoreID().String())
	log.Debugf("sessions store: %s", c.Sessions.GetStoreID().String())
	log.Debugf("teams store: %s", c.Teams.GetStoreID().String())
	log.Debugf("invites store: %s", c.Invites.GetStoreID().String())
	log.Debugf("projects store: %s", c.Projects.GetStoreID().String())
	log.Debugf("tokens store: %s", c.Tokens.GetStoreID().String())
	log.Debugf("users store: %s", c.Users.GetStoreID().String())
	log.Debugf("buckets store: %s", c.Buckets.GetStoreID().String())

	return c, nil
}

func (c *Collections) addCollection(ctx context.Context, col Collection, key datastore.Key) (*uuid.UUID, error) {
	storeID, err := storeIDAtKey(c.ds, key)
	if err != nil {
		return nil, err
	}
	if storeID == nil {
		ids, err := c.threads.NewStore(ctx)
		if err != nil {
			return nil, err
		}
		id := uuid.MustParse(ids)
		storeID = &id

		schema, err := json.Marshal(jsonschema.Reflect(col.GetInstance()))
		if err != nil {
			panic(err)
		}
		if err = c.threads.RegisterSchema(
			ctx,
			storeID.String(),
			col.GetName(),
			string(schema),
			col.GetIndexes()...); err != nil {
			return nil, err
		}
		if err = c.ds.Put(key, storeID[:]); err != nil {
			return nil, err
		}
		if err = c.threads.Start(ctx, storeID.String()); err != nil {
			return nil, err
		}
	}
	return storeID, nil
}

func storeIDAtKey(ds datastore.Datastore, key datastore.Key) (*uuid.UUID, error) {
	idv, err := ds.Get(key)
	if err != nil {
		if errors.Is(err, datastore.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	id := &uuid.UUID{}
	if err = id.UnmarshalBinary(idv); err != nil {
		return nil, err
	}
	return id, nil
}

type authKey string

func AuthCtx(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, authKey("token"), token)
}

type TokenAuth struct{}

func (t TokenAuth) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	md := map[string]string{}
	token, ok := ctx.Value(authKey("token")).(string)
	if ok && token != "" {
		md["Authorization"] = "Bearer " + token
	}
	return md, nil
}

func (t TokenAuth) RequireTransportSecurity() bool {
	return false
}
