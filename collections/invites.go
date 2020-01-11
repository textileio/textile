package collections

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
)

var (
	inviteDur = time.Hour * 24 * 7 * 30
)

type Invite struct {
	ID      string
	TeamID  string
	FromID  string
	ToEmail string
	Expiry  int
}

type Invites struct {
	threads *client.Client
	storeID *uuid.UUID
	token   string
}

func (i *Invites) GetName() string {
	return "Invite"
}

func (i *Invites) GetInstance() interface{} {
	return &Invite{}
}

func (i *Invites) GetStoreID() *uuid.UUID {
	return i.storeID
}

func (i *Invites) Create(ctx context.Context, teamID, fromID, toEmail string) (*Invite, error) {
	ctx = AuthCtx(ctx, i.token)
	invite := &Invite{
		TeamID:  teamID,
		FromID:  fromID,
		ToEmail: toEmail,
		Expiry:  int(time.Now().Add(inviteDur).Unix()),
	}
	if err := i.threads.ModelCreate(ctx, i.storeID.String(), i.GetName(), invite); err != nil {
		return nil, err
	}
	return invite, nil
}

func (i *Invites) Get(ctx context.Context, id string) (*Invite, error) {
	ctx = AuthCtx(ctx, i.token)
	invite := &Invite{}
	if err := i.threads.ModelFindByID(ctx, i.storeID.String(), i.GetName(), id, invite); err != nil {
		return nil, err
	}
	return invite, nil
}

func (i *Invites) Delete(ctx context.Context, id string) error {
	ctx = AuthCtx(ctx, i.token)
	return i.threads.ModelDelete(ctx, i.storeID.String(), i.GetName(), id)
}
