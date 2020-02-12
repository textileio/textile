package collections

import (
	"context"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
)

type Token struct {
	ID        string
	ProjectID string
}

type Tokens struct {
	threads *client.Client
	storeID *uuid.UUID
	token   string
}

func (a *Tokens) GetName() string {
	return "Token"
}

func (a *Tokens) GetInstance() interface{} {
	return &Token{}
}

func (a *Tokens) GetIndexes() []*s.IndexConfig {
	return []*s.IndexConfig{}
}

func (a *Tokens) GetStoreID() *uuid.UUID {
	return a.storeID
}

func (a *Tokens) Create(ctx context.Context, projectID string) (*Token, error) {
	ctx = AuthCtx(ctx, a.token)
	token := &Token{
		ProjectID: projectID,
	}
	if err := a.threads.ModelCreate(ctx, a.storeID.String(), a.GetName(), token); err != nil {
		return nil, err
	}
	return token, nil
}

func (a *Tokens) Get(ctx context.Context, id string) (*Token, error) {
	ctx = AuthCtx(ctx, a.token)
	token := &Token{}
	if err := a.threads.ModelFindByID(ctx, a.storeID.String(), a.GetName(), id, token); err != nil {
		return nil, err
	}
	return token, nil
}

func (a *Tokens) List(ctx context.Context, projectID string) ([]*Token, error) {
	ctx = AuthCtx(ctx, a.token)
	query := s.JSONWhere("ProjectID").Eq(projectID)
	res, err := a.threads.ModelFind(ctx, a.storeID.String(), a.GetName(), query, []*Token{})
	if err != nil {
		return nil, err
	}
	return res.([]*Token), nil
}

func (a *Tokens) Delete(ctx context.Context, id string) error {
	ctx = AuthCtx(ctx, a.token)
	return a.threads.ModelDelete(ctx, a.storeID.String(), a.GetName(), id)
}
