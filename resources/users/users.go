package users

import (
	"github.com/google/uuid"

	"github.com/textileio/go-textile-threads/api/client"
	es "github.com/textileio/go-textile-threads/eventstore"
)

type User struct {
	ID    string
	Email string
	Token string
}

type Users struct {
	threads *client.Client
	storeID *uuid.UUID
}

func (u *Users) GetName() string {
	return "User"
}

func (u *Users) GetInstance() interface{} {
	return &User{}
}

func (u *Users) SetThreads(threads *client.Client) {
	u.threads = threads
}

func (u *Users) GetStoreID() *uuid.UUID {
	return u.storeID
}

func (u *Users) SetStoreID(id *uuid.UUID) {
	u.storeID = id
}

func (u *Users) Create(user *User) error {
	return u.threads.ModelCreate(u.storeID.String(), u.GetName(), user)
}

func (u *Users) Get(id string) (*User, error) {
	user := &User{}
	if err := u.threads.ModelFindByID(u.storeID.String(), u.GetName(), id, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (u *Users) GetByEmail(email string) ([]*User, error) {
	query := es.JSONWhere("Email").Eq(email)
	rawResults, err := u.threads.ModelFind(u.storeID.String(), u.GetName(), query, []*User{})
	if err != nil {
		return nil, err
	}
	users := rawResults.([]*User)
	return users, nil
}

func (u *Users) List() ([]*User, error) {
	res, err := u.threads.ModelFind(u.storeID.String(), u.GetName(), &es.JSONQuery{}, &User{})
	if err != nil {
		return nil, err
	}
	return res.([]*User), nil
}

func (u *Users) Update(user *User) error {
	return u.threads.ModelSave(u.storeID.String(), u.GetName(), user)
}

func (u *Users) Delete(id string) error {
	return u.threads.ModelDelete(u.storeID.String(), u.GetName(), id)
}
