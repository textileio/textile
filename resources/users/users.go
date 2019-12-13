package users

import (
	"encoding/json"

	"github.com/alecthomas/jsonschema"
	"github.com/textileio/go-textile-threads/api/client"
)

type User struct {
	ID string
}

type Users struct {
	storeID string
	schema  *jsonschema.Schema
	threads *client.Client
}

func NewUsers(storeID string, threads *client.Client) (*Users, error) {
	var schema *jsonschema.Schema
	if storeID == "" {
		var err error
		storeID, err = threads.NewStore()
		if err != nil {
			return nil, err
		}
		schema = jsonschema.Reflect(&User{})
		schemaBytes, err := json.Marshal(schema)
		if err != nil {
			panic(err)
		}
		if err = threads.RegisterSchema(storeID, "User", string(schemaBytes)); err != nil {
			return nil, err
		}
	}
	return &Users{
		storeID: storeID,
		schema:  schema,
		threads: threads,
	}, nil
}

func (r *Users) StoreID() string {
	return r.storeID
}

func (r *Users) Create(docs ...interface{}) error {
	return nil
}

func (r *Users) Get(id string, doc interface{}) error {
	return nil
}

func (r *Users) List(dummy interface{}) (interface{}, error) {
	return nil, nil
}

func (r *Users) Update(doc interface{}) error {
	return nil
}

func (r *Users) Delete(id string) error {
	return nil
}
