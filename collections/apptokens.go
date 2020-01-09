package collections

import (
	"context"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
)

type AppToken struct {
	ID        string
	ProjectID string
}

type AppTokens struct {
	threads *client.Client
	storeID *uuid.UUID
}

func (a *AppTokens) GetName() string {
	return "AppToken"
}

func (a *AppTokens) GetInstance() interface{} {
	return &AppToken{}
}

func (a *AppTokens) GetStoreID() *uuid.UUID {
	return a.storeID
}

func (a *AppTokens) Create(ctx context.Context, projectID string) (*AppToken, error) {
	token := &AppToken{
		ProjectID: projectID,
	}
	if err := a.threads.ModelCreate(ctx, a.storeID.String(), a.GetName(), token); err != nil {
		return nil, err
	}
	return token, nil
}

func (a *AppTokens) Get(ctx context.Context, id string) (*AppToken, error) {
	token := &AppToken{}
	if err := a.threads.ModelFindByID(ctx, a.storeID.String(), a.GetName(), id, token); err != nil {
		return nil, err
	}
	return token, nil
}

func (a *AppTokens) List(ctx context.Context, projectID string) ([]*AppToken, error) {
	query := s.JSONWhere("ProjectID").Eq(projectID)
	res, err := a.threads.ModelFind(ctx, a.storeID.String(), a.GetName(), query, []*AppToken{})
	if err != nil {
		return nil, err
	}
	return res.([]*AppToken), nil
}

func (a *AppTokens) Delete(ctx context.Context, id string) error {
	return a.threads.ModelDelete(ctx, a.storeID.String(), a.GetName(), id)
}
