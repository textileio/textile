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

	dsUsersKey    = datastore.NewKey("/users")
	dsSessionsKey = datastore.NewKey("/sessions")
	dsTeamsKey    = datastore.NewKey("/teams")
	dsInvitesKey  = datastore.NewKey("/invites")
	dsProjectsKey = datastore.NewKey("/projects")

	dsAppTokensKey = datastore.NewKey("/apptokens")
	dsAppUsersKey  = datastore.NewKey("/appusers")

	dsFoldersKey = datastore.NewKey("/folders")
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

	Users    *Users
	Sessions *Sessions
	Teams    *Teams
	Invites  *Invites
	Projects *Projects

	AppTokens *AppTokens
	AppUsers  *AppUsers

	Folders *Folders
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

		Users:    &Users{threads: threads, token: token},
		Sessions: &Sessions{threads: threads, token: token},
		Teams:    &Teams{threads: threads, token: token},
		Invites:  &Invites{threads: threads, token: token},
		Projects: &Projects{threads: threads, token: token},

		AppTokens: &AppTokens{threads: threads, token: token},
		AppUsers:  &AppUsers{threads: threads, token: token},

		Folders: &Folders{threads: threads, token: token},
	}
	ctx = AuthCtx(ctx, c.token)

	c.Users.storeID, err = c.addCollection(ctx, c.Users, dsUsersKey)
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
	c.AppTokens.storeID, err = c.addCollection(ctx, c.AppTokens, dsAppTokensKey)
	if err != nil {
		return nil, err
	}
	c.AppUsers.storeID, err = c.addCollection(ctx, c.AppUsers, dsAppUsersKey)
	if err != nil {
		return nil, err
	}
	c.Folders.storeID, err = c.addCollection(ctx, c.Folders, dsFoldersKey)
	if err != nil {
		return nil, err
	}

	log.Debugf("users store: %s", c.Users.GetStoreID().String())
	log.Debugf("sessions store: %s", c.Sessions.GetStoreID().String())
	log.Debugf("teams store: %s", c.Teams.GetStoreID().String())
	log.Debugf("invites store: %s", c.Invites.GetStoreID().String())
	log.Debugf("projects store: %s", c.Projects.GetStoreID().String())
	log.Debugf("app tokens store: %s", c.Projects.GetStoreID().String())
	log.Debugf("app users store: %s", c.Invites.GetStoreID().String())
	log.Debugf("folders store: %s", c.Folders.GetStoreID().String())

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
