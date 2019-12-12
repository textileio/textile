package users

import (
	"encoding/json"

	"github.com/alecthomas/jsonschema"
	"github.com/google/uuid"
	"github.com/textileio/go-textile-core/store"
)

type User struct {
	ID uuid.UUID
}

func Schema() []byte {
	schema, err := json.Marshal(jsonschema.Reflect(&User{}))
	if err != nil {
		panic(err)
	}
	return schema
}

func (u *User) Marshal() ([]byte, error) {
	return json.Marshal(u)
}

func New() (*User, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	return &User{ID: id}, nil
}

func Get(id store.EntityID) (*User, error) {
	return nil, nil
}

func List() ([]User, error) {
	return nil, nil
}

func Update(id store.EntityID) error {
	return nil
}

func Delete(id store.EntityID) error {
	return nil
}
