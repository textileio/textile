package collections

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
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
	token   string
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

func (u *Users) Create(ctx context.Context, email string) (*User, error) {
	ctx = AuthCtx(ctx, u.token)
	user := &User{
		Email:   email,
		Teams:   []string{},
		Created: time.Now().Unix(),
	}
	if err := u.threads.ModelCreate(ctx, u.storeID.String(), u.GetName(), user); err != nil {
		return nil, err
	}
	return user, nil
}

func (u *Users) Get(ctx context.Context, id string) (*User, error) {
	ctx = AuthCtx(ctx, u.token)
	user := &User{}
	if err := u.threads.ModelFindByID(ctx, u.storeID.String(), u.GetName(), id, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (u *Users) GetByEmail(ctx context.Context, email string) ([]*User, error) {
	ctx = AuthCtx(ctx, u.token)
	query := s.JSONWhere("Email").Eq(email)
	res, err := u.threads.ModelFind(ctx, u.storeID.String(), u.GetName(), query, []*User{})
	if err != nil {
		return nil, err
	}
	return res.([]*User), nil
}

func (u *Users) ListByTeam(ctx context.Context, teamID string) ([]*User, error) {
	ctx = AuthCtx(ctx, u.token)
	res, err := u.threads.ModelFind(ctx, u.storeID.String(), u.GetName(), &s.JSONQuery{}, []*User{})
	if err != nil {
		return nil, err
	}
	// @todo: Enable indexes :)
	// @todo: Enable a 'contains' query condition.
	var users []*User
loop:
	for _, u := range res.([]*User) {
		for _, t := range u.Teams {
			if t == teamID {
				users = append(users, u)
				continue loop
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

func (u *Users) JoinTeam(ctx context.Context, user *User, teamID string) error {
	ctx = AuthCtx(ctx, u.token)
	for _, t := range user.Teams {
		if t == teamID {
			return nil
		}
	}
	user.Teams = append(user.Teams, teamID)
	return u.threads.ModelSave(ctx, u.storeID.String(), u.GetName(), user)
}

func (u *Users) LeaveTeam(ctx context.Context, user *User, teamID string) error {
	ctx = AuthCtx(ctx, u.token)
	n := 0
	for _, x := range user.Teams {
		if x != teamID {
			user.Teams[n] = x
			n++
		}
	}
	user.Teams = user.Teams[:n]
	return u.threads.ModelSave(ctx, u.storeID.String(), u.GetName(), user)
}

// @todo: Add a destroy method that calls this. User must first delete projects and teams they own.
func (u *Users) Delete(ctx context.Context, id string) error {
	ctx = AuthCtx(ctx, u.token)
	return u.threads.ModelDelete(ctx, u.storeID.String(), u.GetName(), id)
}
