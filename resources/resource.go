package resources

import (
	"encoding/json"

	"github.com/alecthomas/jsonschema"
	"github.com/google/uuid"
	"github.com/textileio/go-textile-threads/api/client"
)

type Resource interface {
	GetName() string
	GetInstance() interface{}
	SetThreads(threads *client.Client)
	GetStoreID() *uuid.UUID
	SetStoreID(id *uuid.UUID)
}

func AddResource(threads *client.Client, res ...Resource) error {
	for _, r := range res {
		if r.GetStoreID() == nil {
			var err error
			storeID, err := threads.NewStore()
			if err != nil {
				return err
			}
			schema, err := json.Marshal(jsonschema.Reflect(r.GetInstance()))
			if err != nil {
				panic(err)
			}
			if err = threads.RegisterSchema(storeID, r.GetName(), string(schema)); err != nil {
				return err
			}
			id := uuid.MustParse(storeID)
			r.SetStoreID(&id)
		}
		r.SetThreads(threads)
	}
	return nil
}
