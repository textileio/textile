package collections

import (
	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	es "github.com/textileio/go-threads/eventstore"
)

type Team struct {
	ID      string
	OwnerID string
	Name    string
}

type Teams struct {
	threads *client.Client
	storeID *uuid.UUID
}

func (t *Teams) GetName() string {
	return "Team"
}

func (t *Teams) GetInstance() interface{} {
	return &Team{}
}

func (t *Teams) GetStoreID() *uuid.UUID {
	return t.storeID
}

func (t *Teams) Create(team *Team) error {
	return t.threads.ModelCreate(t.storeID.String(), t.GetName(), team)
}

func (t *Teams) Get(id string) (*Team, error) {
	team := &Team{}
	if err := t.threads.ModelFindByID(t.storeID.String(), t.GetName(), id, team); err != nil {
		return nil, err
	}
	return team, nil
}

func (t *Teams) List(ownerID string) ([]*Team, error) {
	query := es.JSONWhere("OwnerID").Eq(ownerID)
	res, err := t.threads.ModelFind(t.storeID.String(), t.GetName(), query, &Team{})
	if err != nil {
		return nil, err
	}
	return res.([]*Team), nil
}

func (t *Teams) Update(team *Team) error {
	return t.threads.ModelSave(t.storeID.String(), t.GetName(), team)
}

func (t *Teams) Delete(id string) error {
	return t.threads.ModelDelete(t.storeID.String(), t.GetName(), id)
}
