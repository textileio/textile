package collections

import (
	"encoding/json"
	"errors"

	"github.com/alecthomas/jsonschema"
	"github.com/google/uuid"
	"github.com/ipfs/go-datastore"
	"github.com/textileio/go-threads/api/client"
)

type Collection interface {
	GetName() string
	GetInstance() interface{}
	SetThreads(*client.Client)
	GetStoreID() *uuid.UUID
	SetStoreID(*uuid.UUID)
}

func AddCollection(threads *client.Client, ds datastore.Datastore, key datastore.Key, col Collection) error {
	storeID, err := storeIDAtKey(ds, key)
	if err != nil {
		return err
	}
	if storeID == nil {
		ids, err := threads.NewStore()
		if err != nil {
			return err
		}
		id := uuid.MustParse(ids)
		storeID = &id

		schema, err := json.Marshal(jsonschema.Reflect(col.GetInstance()))
		if err != nil {
			panic(err)
		}
		if err = threads.RegisterSchema(storeID.String(), col.GetName(), string(schema)); err != nil {
			return err
		}
		if err = ds.Put(key, storeID[:]); err != nil {
			return err
		}
		if err = threads.Start(storeID.String()); err != nil {
			return err
		}
	}
	col.SetThreads(threads)
	col.SetStoreID(storeID)
	return nil
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
