package collections

import (
	"time"

	"github.com/google/uuid"

	"github.com/textileio/go-threads/api/client"
	es "github.com/textileio/go-threads/eventstore"
)

type Project struct {
	ID      string
	Name    string
	Scope   string // user or team
	StoreID string
	Created int64
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

func (p *Projects) Create(name, scope string) (*Project, error) {
	proj := &Project{
		Name:    name,
		Scope:   scope,
		Created: time.Now().Unix(),
	}
	// Create a dedicated store for the project
	var err error
	proj.StoreID, err = p.threads.NewStore()
	if err != nil {
		return nil, err
	}
	if err := p.threads.ModelCreate(p.storeID.String(), p.GetName(), proj); err != nil {
		return nil, err
	}
	return proj, nil
}

func (p *Projects) Get(id string) (*Project, error) {
	proj := &Project{}
	if err := p.threads.ModelFindByID(p.storeID.String(), p.GetName(), id, proj); err != nil {
		return nil, err
	}
	return proj, nil
}

func (p *Projects) List(scope string) ([]*Project, error) {
	query := es.JSONWhere("Scope").Eq(scope)
	res, err := p.threads.ModelFind(p.storeID.String(), p.GetName(), query, &Project{})
	if err != nil {
		return nil, err
	}
	return res.([]*Project), nil
}

func (p *Projects) Delete(id string) error {
	return p.threads.ModelDelete(p.storeID.String(), p.GetName(), id)
}
