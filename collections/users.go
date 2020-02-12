package collections

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
	"google.golang.org/grpc/status"
)

type User struct {
	ID        string
	ProjectID string
	StoreID   string
	Created   int64
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

func (u *Users) GetIndexes() []*s.IndexConfig {
	return []*s.IndexConfig{}
}

func (u *Users) GetStoreID() *uuid.UUID {
	return u.storeID
}

func (u *Users) GetOrCreate(ctx context.Context, projectID, deviceID string) (user *User, err error) {
	ctx = AuthCtx(ctx, u.token)
	user, err = u.Get(ctx, deviceID)
	if user != nil {
		return
	}
	if err != nil {
		stat, ok := status.FromError(err)
		if !ok {
			return
		}
		if stat.Message() != s.ErrNotFound.Error() {
			return
		}
	}
	user = &User{
		ID:        deviceID,
		ProjectID: projectID,
		Created:   time.Now().Unix(),
	}
	user.StoreID, err = u.threads.NewStore(ctx)
	if err != nil {
		return nil, err
	}
	if err = u.threads.ModelCreate(ctx, u.storeID.String(), u.GetName(), user); err != nil {
		return nil, err
	}
	if err = u.threads.Start(ctx, user.StoreID); err != nil {
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

func (u *Users) List(ctx context.Context, projectID string) ([]*User, error) {
	ctx = AuthCtx(ctx, u.token)
	query := s.JSONWhere("ProjectID").Eq(projectID)
	res, err := u.threads.ModelFind(ctx, u.storeID.String(), u.GetName(), query, []*User{})
	if err != nil {
		return nil, err
	}
	return res.([]*User), nil
}

// @todo: Delete associated sessions
func (u *Users) Delete(ctx context.Context, id string) error {
	ctx = AuthCtx(ctx, u.token)
	return u.threads.ModelDelete(ctx, u.storeID.String(), u.GetName(), id)
}
