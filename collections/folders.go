package collections

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
)

type Folder struct {
	ID        string
	Path      string
	Name      string
	Public    bool
	ProjectID string
	Created   int64
}

type Folders struct {
	threads *client.Client
	storeID *uuid.UUID
	token   string
}

func (f *Folders) GetName() string {
	return "Folder"
}

func (f *Folders) GetInstance() interface{} {
	return &Folder{}
}

func (f *Folders) GetIndexes() []*s.IndexConfig {
	return []*s.IndexConfig{{
		Path: "Path",
	}, {
		Path:   "Name",
		Unique: true,
	}, {
		Path: "ProjectID",
	}}
}

func (f *Folders) GetStoreID() *uuid.UUID {
	return f.storeID
}

func (f *Folders) Create(
	ctx context.Context,
	pth path.Resolved,
	name string,
	public bool,
	projectID string,
) (*Folder, error) {
	ctx = AuthCtx(ctx, f.token)
	folder := &Folder{
		Path:      pth.String(),
		Name:      name,
		Public:    public,
		ProjectID: projectID,
		Created:   time.Now().Unix(),
	}
	if err := f.threads.ModelCreate(ctx, f.storeID.String(), f.GetName(), folder); err != nil {
		return nil, err
	}
	return folder, nil
}

func (f *Folders) GetByName(ctx context.Context, name string) (*Folder, error) {
	ctx = AuthCtx(ctx, f.token)
	query := s.JSONWhere("Name").Eq(name)
	res, err := f.threads.ModelFind(ctx, f.storeID.String(), f.GetName(), query, []*Folder{})
	if err != nil {
		return nil, err
	}
	folders := res.([]*Folder)
	if len(folders) == 0 {
		return nil, nil
	}
	return folders[0], nil
}

func (f *Folders) List(ctx context.Context, projectID string) ([]*Folder, error) {
	ctx = AuthCtx(ctx, f.token)
	query := s.JSONWhere("ProjectID").Eq(projectID)
	res, err := f.threads.ModelFind(ctx, f.storeID.String(), f.GetName(), query, []*Folder{})
	if err != nil {
		return nil, err
	}
	return res.([]*Folder), nil
}

func (f *Folders) UpdatePath(ctx context.Context, folder *Folder, pth path.Resolved) error {
	ctx = AuthCtx(ctx, f.token)
	folder.Path = pth.String()
	return f.threads.ModelSave(ctx, f.storeID.String(), f.GetName(), folder)
}

func (f *Folders) Delete(ctx context.Context, id string) error {
	ctx = AuthCtx(ctx, f.token)
	return f.threads.ModelDelete(ctx, f.storeID.String(), f.GetName(), id)
}
