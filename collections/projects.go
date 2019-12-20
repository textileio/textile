package collections

import (
	"github.com/google/uuid"

	"github.com/textileio/go-threads/api/client"
	es "github.com/textileio/go-threads/eventstore"
)

type Project struct {
	ID      string
	Name    string
	ScopeID string // user or team
	StoreID string
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

func (p *Projects) SetThreads(threads *client.Client) {
	p.threads = threads
}

func (p *Projects) GetStoreID() *uuid.UUID {
	return p.storeID
}

func (p *Projects) SetStoreID(id *uuid.UUID) {
	p.storeID = id
}

func (p *Projects) Create(proj *Project) error {
	// Create a dedicated store for the project
	var err error
	proj.StoreID, err = p.threads.NewStore()
	if err != nil {
		return err
	}
	return p.threads.ModelCreate(p.storeID.String(), p.GetName(), proj)
}

func (p *Projects) Get(id string) (*Project, error) {
	proj := &Project{}
	if err := p.threads.ModelFindByID(p.storeID.String(), p.GetName(), id, proj); err != nil {
		return nil, err
	}
	return proj, nil
}

func (p *Projects) List(groupID string) ([]*Project, error) {
	query := es.JSONWhere("ScopeID").Eq(groupID)
	res, err := p.threads.ModelFind(p.storeID.String(), p.GetName(), query, &Project{})
	if err != nil {
		return nil, err
	}
	return res.([]*Project), nil
}

func (p *Projects) Update(proj *Project) error {
	return p.threads.ModelSave(p.storeID.String(), p.GetName(), proj)
}

func (p *Projects) Delete(id string) error {
	return p.threads.ModelDelete(p.storeID.String(), p.GetName(), id)
}
