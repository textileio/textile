package dns

import (
	"fmt"

	cf "github.com/cloudflare/cloudflare-go"
	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/util"
)

var (
	log = logging.Logger("dns")
)

const IPFSGateway = "cloudflare-ipfs.com"

// Manager wraps a CloudflareClient client.
type Manager struct {
	Domain string

	api    *cf.API
	zoneID string
}

// NewManager return a cloudflare-backed dns updating client.
func NewManager(domain string, zoneID string, token string, debug bool) (*Manager, error) {
	if debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"dns": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	api, err := cf.NewWithAPIToken(token)
	if err != nil {
		return nil, err
	}
	return &Manager{
		Domain: domain,
		api:    api,
		zoneID: zoneID,
	}, nil
}

// NewCNAME enters a new dns record for a CNAME.
func (m *Manager) NewCNAME(name string, target string) (*cf.DNSRecord, error) {
	res, err := m.api.CreateDNSRecord(m.zoneID, cf.DNSRecord{
		Type:    "CNAME",
		Name:    name,
		Content: target,
		Proxied: false,
	})
	if err != nil {
		return nil, err
	}
	log.Debugf("created CNAME record %s -> %s", name, target)
	return &res.Result, nil
}

// NewTXT enters a new dns record for a TXT.
func (m *Manager) NewTXT(name string, content string) (*cf.DNSRecord, error) {
	res, err := m.api.CreateDNSRecord(m.zoneID, cf.DNSRecord{
		Type:    "TXT",
		Name:    name,
		Content: content,
	})
	if err != nil {
		return nil, err
	}
	log.Debugf("created TXT record %s -> %s", name, content)
	return &res.Result, nil
}

// NewDNSLink enters a two dns records to enable DNS link.
func (m *Manager) NewDNSLink(subdomain string, hash string) ([]*cf.DNSRecord, error) {
	cname, err := m.NewCNAME(subdomain, IPFSGateway)
	if err != nil {
		return nil, err
	}

	name := CreateDNSLinkName(subdomain)
	content := CreateDNSLinkContent(hash)
	txt, err := m.NewTXT(name, content)
	if err != nil {
		// Cleanup the orphaned cname record
		_ = m.DeleteRecord(cname.ID)
		return nil, err
	}

	log.Debugf("created DNSLink record %s -> %s", subdomain, hash)
	return []*cf.DNSRecord{cname, txt}, nil
}

// UpdateRecord updates an existing record.
func (m *Manager) UpdateRecord(id, rtype, name, content string) error {
	if err := m.api.UpdateDNSRecord(m.zoneID, id, cf.DNSRecord{
		Type:    rtype,
		Name:    name,
		Content: content,
	}); err != nil {
		return err
	}
	log.Debugf("updated record %s -> %s", name, content)
	return nil
}

// Delete removes a record by ID from dns.
func (m *Manager) DeleteRecord(id string) error {
	if err := m.api.DeleteDNSRecord(m.zoneID, id); err != nil {
		return err
	}
	log.Debugf("deleted record %s", id)
	return nil
}

// CreateDNSLinkName converts a subdomain into the Name format for dnslink TXT entries.
func CreateDNSLinkName(subdomain string) string {
	return fmt.Sprintf("_dnslink.%s", subdomain)
}

// CreateDNSLinkContent converts a hash into the Content format for dnslink TXT entries.
func CreateDNSLinkContent(hash string) string {
	return fmt.Sprintf("dnslink=/ipfs/%s", hash)
}
