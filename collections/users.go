package collections

import (
	"github.com/google/uuid"

	"github.com/textileio/go-threads/api/client"
	es "github.com/textileio/go-threads/eventstore"
)

type User struct {
	ID    string
	Email string
	Teams []string
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

func (u *Users) GetStoreID() *uuid.UUID {
	return u.storeID
}

func (u *Users) Create(email string) (*User, error) {
	user := &User{Email: email, Teams: []string{}}
	if err := u.threads.ModelCreate(u.storeID.String(), u.GetName(), user); err != nil {
		return nil, err
	}
	return user, nil
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

func (u *Users) HasTeam(user *User, team *Team) bool {
	for _, t := range user.Teams {
		if team.ID == t {
			return true
		}
	}
	return false
}

func (u *Users) AddTeam(user *User, team *Team) error {
	user.Teams = append(user.Teams, team.ID)
	return u.threads.ModelSave(u.storeID.String(), u.GetName(), user)
}

func (u *Users) Delete(id string) error {
	return u.threads.ModelDelete(u.storeID.String(), u.GetName(), id)
}
