package collections

import (
	"context"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
)

type AppUser struct {
	ID        string
	ProjectID string
	DeviceID  string
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

func (u *AppUsers) Create(ctx context.Context, projectID, deviceID string) (*AppUser, error) {
	user := &AppUser{
		ProjectID: projectID,
		DeviceID:  deviceID,
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

// @todo: Should be unique
func (u *AppUsers) GetByDevice(ctx context.Context, deviceID string) ([]*AppUser, error) {
	query := s.JSONWhere("DeviceID").Eq(deviceID)
	res, err := u.threads.ModelFind(ctx, u.storeID.String(), u.GetName(), query, []*AppUser{})
	if err != nil {
		return nil, err
	}
	return res.([]*AppUser), nil
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
