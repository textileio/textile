package collections

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
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
	token   string
}

func (t *Teams) GetName() string {
	return "Team"
}

func (t *Teams) GetInstance() interface{} {
	return &Team{}
}

func (t *Teams) GetIndexes() []*s.IndexConfig {
	return []*s.IndexConfig{{
		Path: "OwnerID",
	}, {
		Path:   "Name",
		Unique: true,
	}}
}

func (t *Teams) GetStoreID() *uuid.UUID {
	return t.storeID
}

func (t *Teams) Create(ctx context.Context, ownerID, name string) (*Team, error) {
	validName, err := toValidName(name)
	if err != nil {
		return nil, err
	}
	ctx = AuthCtx(ctx, t.token)
	team := &Team{
		OwnerID: ownerID,
		Name:    validName,
		Created: time.Now().Unix(),
	}
	if err := t.threads.ModelCreate(ctx, t.storeID.String(), t.GetName(), team); err != nil {
		return nil, err
	}
	return team, nil
}

func (t *Teams) Get(ctx context.Context, id string) (*Team, error) {
	ctx = AuthCtx(ctx, t.token)
	team := &Team{}
	if err := t.threads.ModelFindByID(ctx, t.storeID.String(), t.GetName(), id, team); err != nil {
		return nil, err
	}
	return team, nil
}

func (t *Teams) Delete(ctx context.Context, id string) error {
	ctx = AuthCtx(ctx, t.token)
	return t.threads.ModelDelete(ctx, t.storeID.String(), t.GetName(), id)
}
