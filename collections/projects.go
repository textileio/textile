package collections

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
)

type Project struct {
	ID              string
	Name            string
	Scope           string // user or team
	StoreID         string
	Created         int64
	FCWalletAddress string
}

type Projects struct {
	threads *client.Client
	storeID *uuid.UUID
}

func (p *Projects) GetName() string {
	return "Project"
}

func (p *Projects) GetInstance() interface{} {
	return &Project{}
}

func (p *Projects) GetStoreID() *uuid.UUID {
	return p.storeID
}

func (p *Projects) Create(ctx context.Context, name, scope, fcWalletAddress string) (*Project, error) {
	proj := &Project{
		Name:            name,
		Scope:           scope,
		Created:         time.Now().Unix(),
		FCWalletAddress: fcWalletAddress,
	}
	// Create a dedicated store for the project
	var err error
	proj.StoreID, err = p.threads.NewStore(ctx)
	if err != nil {
		return nil, err
	}
	if err := p.threads.ModelCreate(ctx, p.storeID.String(), p.GetName(), proj); err != nil {
		return nil, err
	}
	return proj, nil
}

func (p *Projects) Get(ctx context.Context, id string) (*Project, error) {
	proj := &Project{}
	if err := p.threads.ModelFindByID(ctx, p.storeID.String(), p.GetName(), id, proj); err != nil {
		return nil, err
	}
	return proj, nil
}

func (p *Projects) List(ctx context.Context, scope string) ([]*Project, error) {
	query := s.JSONWhere("Scope").Eq(scope)
	res, err := p.threads.ModelFind(ctx, p.storeID.String(), p.GetName(), query, []*Project{})
	if err != nil {
		return nil, err
	}
	return res.([]*Project), nil
}

func (p *Projects) Delete(ctx context.Context, id string) error {
	return p.threads.ModelDelete(ctx, p.storeID.String(), p.GetName(), id)
}
