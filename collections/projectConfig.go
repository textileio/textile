package collections

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
)

// ConfigItem represents a single config value
type ConfigItem struct {
	ID         string
	UniqueName string
	Name       string
	Values     map[string]string
	ProjectID  string
	Created    int64
	Updated    int64
}

// ProjectConfig provides the project config api
type ProjectConfig struct {
	threads *client.Client
	storeID *uuid.UUID
	token   string
}

// GetName returns the entity name
func (p *ProjectConfig) GetName() string {
	return "Config"
}

// GetInstance returns a ConfigItem instance
func (p *ProjectConfig) GetInstance() interface{} {
	return &ConfigItem{}
}

// GetIndexes returns the indexes
func (p *ProjectConfig) GetIndexes() []*s.IndexConfig {
	return []*s.IndexConfig{{
		Path:   "UniqueName",
		Unique: true,
	}, {
		Path: "ProjectID",
	}}
}

// GetStoreID returns the store id
func (p *ProjectConfig) GetStoreID() *uuid.UUID {
	return p.storeID
}

// Create creates a new config
func (p *ProjectConfig) Create(ctx context.Context, name string, values map[string]string, projectID string) (*ConfigItem, error) {
	validName, err := toValidName(name)
	if err != nil {
		return nil, err
	}
	ctx = AuthCtx(ctx, p.token)
	config := &ConfigItem{
		UniqueName: fmt.Sprintf("%v-%v", projectID, validName),
		Name:       validName,
		Values:     values,
		ProjectID:  projectID,
		Created:    time.Now().Unix(),
		Updated:    time.Now().Unix(),
	}
	if err := p.threads.ModelCreate(ctx, p.storeID.String(), p.GetName(), config); err != nil {
		return nil, err
	}
	return config, nil
}

// Get fetches a single config
func (p *ProjectConfig) Get(ctx context.Context, projectID string, name string) (*ConfigItem, error) {
	ctx = AuthCtx(ctx, p.token)
	query := s.JSONWhere("ProjectID").Eq(projectID).JSONAnd("Name").Eq(name)
	res, err := p.threads.ModelFind(ctx, p.storeID.String(), p.GetName(), query, []*ConfigItem{})
	if err != nil {
		return nil, err
	}
	configItems := res.([]*ConfigItem)
	if len(configItems) == 0 {
		return nil, nil
	}
	return configItems[0], nil
}

// GetByID fetches a single config
func (p *ProjectConfig) GetByID(ctx context.Context, ID string) (*ConfigItem, error) {
	ctx = AuthCtx(ctx, p.token)
	configItem := &ConfigItem{}
	if err := p.threads.ModelFindByID(ctx, p.storeID.String(), p.GetName(), ID, configItem); err != nil {
		return nil, err
	}
	return configItem, nil
}

// List lists all ConfigItems for a project
func (p *ProjectConfig) List(ctx context.Context, projectID string) ([]*ConfigItem, error) {
	ctx = AuthCtx(ctx, p.token)
	query := s.JSONWhere("ProjectID").Eq(projectID)
	res, err := p.threads.ModelFind(ctx, p.storeID.String(), p.GetName(), query, []*ConfigItem{})
	if err != nil {
		return nil, err
	}
	return res.([]*ConfigItem), nil
}

// Save saves a config
func (p *ProjectConfig) Save(ctx context.Context, config *ConfigItem) error {
	ctx = AuthCtx(ctx, p.token)
	return p.threads.ModelSave(ctx, p.storeID.String(), p.GetName(), config)
}

// Delete deletes a config
func (p *ProjectConfig) Delete(ctx context.Context, id string) error {
	ctx = AuthCtx(ctx, p.token)
	return p.threads.ModelDelete(ctx, p.storeID.String(), p.GetName(), id)
}
