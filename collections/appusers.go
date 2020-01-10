package collections

import (
	"context"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
	"google.golang.org/grpc/status"
)

type AppUser struct {
	ID        string
	ProjectID string
}

type AppUsers struct {
	threads *client.Client
	storeID *uuid.UUID
}

func (u *AppUsers) GetName() string {
	return "AppUser"
}

func (u *AppUsers) GetInstance() interface{} {
	return &AppUser{}
}

func (u *AppUsers) GetStoreID() *uuid.UUID {
	return u.storeID
}

func (u *AppUsers) GetOrCreate(ctx context.Context, projectID, deviceID string) (user *AppUser, err error) {
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
	user = &AppUser{
		ID:        deviceID,
		ProjectID: projectID,
	}
	if err := u.threads.ModelCreate(ctx, u.storeID.String(), u.GetName(), user); err != nil {
		return nil, err
	}
	return user, nil
}

func (u *AppUsers) Get(ctx context.Context, id string) (*AppUser, error) {
	user := &AppUser{}
	if err := u.threads.ModelFindByID(ctx, u.storeID.String(), u.GetName(), id, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (u *AppUsers) List(ctx context.Context, projectID string) ([]*AppUser, error) {
	query := s.JSONWhere("ProjectID").Eq(projectID)
	res, err := u.threads.ModelFind(ctx, u.storeID.String(), u.GetName(), query, []*AppUser{})
	if err != nil {
		return nil, err
	}
	return res.([]*AppUser), nil
}

// @todo: Delete associated sessions
func (u *AppUsers) Delete(ctx context.Context, id string) error {
	return u.threads.ModelDelete(ctx, u.storeID.String(), u.GetName(), id)
}
