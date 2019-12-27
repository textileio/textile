package collections

import (
	"time"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	es "github.com/textileio/go-threads/eventstore"
)

type User struct {
	ID      string
	Email   string
	Teams   []string
	Created int64
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
	user := &User{
		Email:   email,
		Teams:   []string{},
		Created: time.Now().Unix(),
	}
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

func (u *Users) ListByTeam(teamID string) ([]*User, error) {
	res, err := u.threads.ModelFind(u.storeID.String(), u.GetName(), &es.JSONQuery{}, &User{})
	if err != nil {
		return nil, err
	}
	// @todo: Enable indexes :)
	// @todo: Enable a 'contains' query condition.
	var users []*User
	for _, u := range res.([]*User) {
		for _, t := range u.Teams {
			if t == teamID {
				users = append(users, u)
			}
		}
	}
	return users, nil
}

func (u *Users) HasTeam(user *User, teamID string) bool {
	for _, t := range user.Teams {
		if teamID == t {
			return true
		}
	}
	return false
}

func (u *Users) JoinTeam(user *User, teamID string) error {
	user.Teams = append(user.Teams, teamID)
	return u.threads.ModelSave(u.storeID.String(), u.GetName(), user)
}

func (u *Users) LeaveTeam(user *User, teamID string) error {
	n := 0
	for _, x := range user.Teams {
		if x != teamID {
			user.Teams[n] = x
			n++
		}
	}
	user.Teams = user.Teams[:n]
	return u.threads.ModelSave(u.storeID.String(), u.GetName(), user)
}

// @todo: Add a destroy method that calls this. User must first delete projects and teams they own.
func (u *Users) Delete(id string) error {
	return u.threads.ModelDelete(u.storeID.String(), u.GetName(), id)
}
