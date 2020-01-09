package collections

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/textileio/go-threads/api/client"
	s "github.com/textileio/go-threads/store"
	"github.com/textileio/textile/dns"
)

var defaultHash = "QmSNWjbDkafwxWUGWTyP1ko4J1CW2WeNfeoiCuTUW2YnDY"

type Project struct {
	ID              string
	Name            string
	Scope           string // user or team
	StoreID         string
	Domain          string
	Records         []*dns.Record
	FCWalletAddress string
	Created         int64
}

type Projects struct {
	threads    *client.Client
	storeID    *uuid.UUID
	dnsManager *dns.Manager
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
	// Create subdomain for the project
	domain := ""
	records := []*dns.Record{}
	if p.dnsManager.Started {
		safesd, err := dns.CreateURLSafeSubdomain(name, 8)
		if err != nil {
			return nil, err
		}
		records, err := p.dnsManager.NewDNSLink(safesd, defaultHash)
		if err != nil {
			return nil, err
		}
		domain = p.dnsManager.GetDomain(safesd)
		records = records
	}

	proj := &Project{
		Name:            name,
		Scope:           scope,
		Domain:          domain,
		Records:         records,
		FCWalletAddress: fcWalletAddress,
		Created:         time.Now().Unix(),
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
	proj, err := p.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := p.dnsManager.DeleteRecords(proj.Records); err != nil {
		return err
	}
	return p.threads.ModelDelete(ctx, p.storeID.String(), p.GetName(), id)
}
