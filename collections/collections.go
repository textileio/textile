package collections

import (
	"context"
	"fmt"
	"time"

	"github.com/gosimple/slug"
	logging "github.com/ipfs/go-log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const dbName = "textile"

var (
	_ = logging.Logger("collections")
)

func init() {
	slug.MaxLength = 64
}

type Collections struct {
	m *mongo.Client

	Sessions   *Sessions
	Developers *Developers
	Teams      *Teams
	Invites    *Invites
	Projects   *Projects

	//Tokens *Tokens
	//Users  *Users

	//Buckets *Buckets
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
	teams, err := NewTeams(ctx, db)
	if err != nil {
		return nil, err
	}
	invites, err := NewInvites(ctx, db)
	if err != nil {
		return nil, err
	}
	projects, err := NewProjects(ctx, db)
	if err != nil {
		return nil, err
	}

	return &Collections{
		m: m,

		Sessions:   sessions,
		Developers: developers,
		Teams:      teams,
		Invites:    invites,
		Projects:   projects,

		//Tokens: &Tokens{threads: threads, token: token},
		//Users:  &Users{threads: threads, token: token},

		//Buckets: &Buckets{threads: threads, token: token},
	}, nil
}

func (c *Collections) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return c.m.Disconnect(ctx)
}

//func (c *Collections) addCollection(ctx context.Context, col Collection, key datastore.Key) (*uuid.UUID, error) {
//	storeID, err := storeIDAtKey(c.ds, key)
//	if err != nil {
//		return nil, err
//	}
//	if storeID == nil {
//		ids, err := c.threads.NewStore(ctx)
//		if err != nil {
//			return nil, err
//		}
//		id := uuid.MustParse(ids)
//		storeID = &id
//
//		schema, err := json.Marshal(jsonschema.Reflect(col.GetInstance()))
//		if err != nil {
//			panic(err)
//		}
//		if err = c.threads.RegisterSchema(
//			ctx,
//			storeID.String(),
//			col.GetName(),
//			string(schema),
//			col.GetIndexes()...); err != nil {
//			return nil, err
//		}
//		if err = c.ds.Put(key, storeID[:]); err != nil {
//			return nil, err
//		}
//		if err = c.threads.Start(ctx, storeID.String()); err != nil {
//			return nil, err
//		}
//	}
//	return storeID, nil
//}
//
//func storeIDAtKey(ds datastore.Datastore, key datastore.Key) (*uuid.UUID, error) {
//	idv, err := ds.Get(key)
//	if err != nil {
//		if errors.Is(err, datastore.ErrNotFound) {
//			return nil, nil
//		}
//		return nil, err
//	}
//	id := &uuid.UUID{}
//	if err = id.UnmarshalBinary(idv); err != nil {
//		return nil, err
//	}
//	return id, nil
//}

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

func toValidName(str string) (name string, err error) {
	name = slug.Make(str)
	if len(name) < 3 {
		err = fmt.Errorf("name must contain at least three URL-safe characters")
		return
	}
	return name, nil
}
