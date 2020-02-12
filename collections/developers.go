package collections

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
)

type Developer struct {
	ID      string
	Email   string
	Teams   []string
	Created int64
}

type Developers struct {
	threads *client.Client
	storeID *uuid.UUID
	token   string
}

func (d *Developers) GetName() string {
	return "Developer"
}

func (d *Developers) GetInstance() interface{} {
	return &Developer{}
}

func (d *Developers) GetIndexes() []*s.IndexConfig {
	return []*s.IndexConfig{{
		Path:   "Email",
		Unique: true,
	}}
}

func (d *Developers) GetStoreID() *uuid.UUID {
	return d.storeID
}

func (d *Developers) Create(ctx context.Context, email string) (*Developer, error) {
	ctx = AuthCtx(ctx, d.token)
	dev := &Developer{
		Email:   email,
		Teams:   []string{},
		Created: time.Now().Unix(),
	}
	if err := d.threads.ModelCreate(ctx, d.storeID.String(), d.GetName(), dev); err != nil {
		return nil, err
	}
	return dev, nil
}

func (d *Developers) Get(ctx context.Context, id string) (*Developer, error) {
	ctx = AuthCtx(ctx, d.token)
	dev := &Developer{}
	if err := d.threads.ModelFindByID(ctx, d.storeID.String(), d.GetName(), id, dev); err != nil {
		return nil, err
	}
	return dev, nil
}

func (d *Developers) GetByEmail(ctx context.Context, email string) ([]*Developer, error) {
	ctx = AuthCtx(ctx, d.token)
	query := s.JSONWhere("Email").Eq(email)
	res, err := d.threads.ModelFind(ctx, d.storeID.String(), d.GetName(), query, []*Developer{})
	if err != nil {
		return nil, err
	}
	return res.([]*Developer), nil
}

func (d *Developers) ListByTeam(ctx context.Context, teamID string) ([]*Developer, error) {
	ctx = AuthCtx(ctx, d.token)
	res, err := d.threads.ModelFind(ctx, d.storeID.String(), d.GetName(), &s.JSONQuery{}, []*Developer{})
	if err != nil {
		return nil, err
	}

	// @todo: Enable a 'contains' query condition.
	var devs []*Developer
loop:
	for _, dev := range res.([]*Developer) {
		for _, t := range dev.Teams {
			if t == teamID {
				devs = append(devs, dev)
				continue loop
			}
		}
	}
	return devs, nil
}

func (d *Developers) HasTeam(dev *Developer, teamID string) bool {
	for _, t := range dev.Teams {
		if teamID == t {
			return true
		}
	}
	return false
}

func (d *Developers) JoinTeam(ctx context.Context, dev *Developer, teamID string) error {
	ctx = AuthCtx(ctx, d.token)
	for _, t := range dev.Teams {
		if t == teamID {
			return nil
		}
	}
	dev.Teams = append(dev.Teams, teamID)
	return d.threads.ModelSave(ctx, d.storeID.String(), d.GetName(), dev)
}

func (d *Developers) LeaveTeam(ctx context.Context, dev *Developer, teamID string) error {
	ctx = AuthCtx(ctx, d.token)
	n := 0
	for _, x := range dev.Teams {
		if x != teamID {
			dev.Teams[n] = x
			n++
		}
	}
	dev.Teams = dev.Teams[:n]
	return d.threads.ModelSave(ctx, d.storeID.String(), d.GetName(), dev)
}

// @todo: Developer must first delete projects and teams they own
// @todo: Delete associated sessions
func (d *Developers) Delete(ctx context.Context, id string) error {
	ctx = AuthCtx(ctx, d.token)
	return d.threads.ModelDelete(ctx, d.storeID.String(), d.GetName(), id)
}
