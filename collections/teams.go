package collections

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
)

type Team struct {
	ID      string
	OwnerID string
	Name    string
	Created int64
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

func (t *Teams) Create(ctx context.Context, ownerID, name string) (*Team, error) {
	team := &Team{
		OwnerID: ownerID,
		Name:    name,
		Created: time.Now().Unix(),
	}
	if err := t.threads.ModelCreate(ctx, t.storeID.String(), t.GetName(), team); err != nil {
		return nil, err
	}
	return team, nil
}

func (t *Teams) Get(ctx context.Context, id string) (*Team, error) {
	team := &Team{}
	if err := t.threads.ModelFindByID(ctx, t.storeID.String(), t.GetName(), id, team); err != nil {
		return nil, err
	}
	return team, nil
}

func (t *Teams) Delete(ctx context.Context, id string) error {
	return t.threads.ModelDelete(ctx, t.storeID.String(), t.GetName(), id)
}
