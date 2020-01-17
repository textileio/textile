package dns

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"
	logging "github.com/ipfs/go-log"
	"github.com/textileio/go-threads/util"
)

var (
	log = logging.Logger("dns")
)

const ipfsGateway = "www.cloudflare-ipfs.com" // future could be set to project's gateway

// Manager wraps a CloudflareClient client.
type Manager struct {
	api     *cloudflare.API
	domain  string
	zoneID  string
	debug   bool
	Started bool
}

type Record struct {
	ID       string
	Type     string
	Content  string
	Name     string
	Modified int64
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

	client := &Manager{
		domain: domain,
		debug:  debug,
	}

	if token != "" {
		api, err := cloudflare.NewWithAPIToken(token)
		if err != nil {
			return nil, err
		}
		client.api = api
		client.zoneID = zoneID
		client.Started = true
	}
	return client, nil
}

// NewCNAME enters a new dns record for a CNAME
func (m *Manager) NewCNAME(subdomain string, target string) (*Record, error) {
	// parse err here
	cname, err := m.api.CreateDNSRecord(m.zoneID, cnameRecordInput(subdomain, target))
	if err != nil {
		return nil, err
	}
	return &Record{
		ID:       cname.Result.ID,
		Type:     cname.Result.Type,
		Content:  cname.Result.Content,
		Name:     cname.Result.Name,
		Modified: cname.Result.ModifiedOn.Unix(),
	}, nil
}

// NewTXT enters a new dns record for a TXT
func (m *Manager) NewTXT(name string, content string) (*Record, error) {
	txt, err := m.api.CreateDNSRecord(m.zoneID, txtRecordInput(name, content))
	if err != nil {
		return nil, err
	}
	return &Record{
		ID:       txt.Result.ID,
		Type:     txt.Result.Type,
		Content:  txt.Result.Content,
		Name:     txt.Result.Name,
		Modified: txt.Result.ModifiedOn.Unix(),
	}, nil
}

// NewDNSLink enters a two dns records to enable DNS link
func (m *Manager) NewDNSLink(subdomain string, hash string) ([]*Record, error) {
	cname, err := m.NewCNAME(subdomain, ipfsGateway)
	if err != nil {
		return nil, err
	}

	name := CreateDNSLinkName(subdomain)
	content := CreateDNSLinkContent(hash)
	txt, err := m.NewTXT(name, content)
	if err != nil {
		// cleanup the orphaned cname record
		m.Delete(cname.ID)
		return nil, err
	}

	return []*Record{
		cname,
		txt,
	}, nil
}

// UpdateRecord updates an existing record
func (m *Manager) UpdateRecord(newRecord *Record) error {
	update := cloudflare.DNSRecord{
		Type:    newRecord.Type,
		Name:    newRecord.Name,
		Content: newRecord.Content,
	}
	return m.api.UpdateDNSRecord(m.zoneID, newRecord.ID, update)
}

// GetDomain returns fully resolvable domain
func (m *Manager) GetDomain(subdomain string) string {
	return fmt.Sprintf("https://%s.%s/", subdomain, m.domain)
}

// DeleteRecords deletes an array of records
func (m *Manager) DeleteRecords(records []*Record) error {
	for _, record := range records {
		if err := m.Delete(record.ID); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes a record by ID from dns
func (m *Manager) Delete(recordID string) error {
	return m.api.DeleteDNSRecord(m.zoneID, recordID)
}

//CreateDNSLinkName converts a subdomain into the Name format for dnslink TXT entries
func CreateDNSLinkName(subdomain string) string {
	return fmt.Sprintf("_dnslink.%s", subdomain)
}

//CreateDNSLinkContent converts a hash into the Content format for dnslink TXT entries
func CreateDNSLinkContent(hash string) string {
	return fmt.Sprintf("dnslink=/ipfs/%s", hash)
}

//CreateURLSafeSubdomain returns a url safe subdomain with optional suffix
func CreateURLSafeSubdomain(subdomain string, suffix int) (string, error) {
	sfx := ""
	if suffix > 0 {
		charset := "abcdefghijklmnopqrstuvwxyz0123456789"

		var seededRand *rand.Rand = rand.New(
			rand.NewSource(time.Now().UnixNano()))

		b := make([]byte, suffix)
		for i := range b {
			b[i] = charset[seededRand.Intn(len(charset))]
		}
		sfx = fmt.Sprintf("-%s", string(b))
	}
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		return "", nil
	}
	safestr := reg.ReplaceAllString(strings.ToLower(subdomain), "")
	return fmt.Sprintf("%s%s", safestr, sfx), nil
}

// txtRecordInput generates a cloudflare.DNSRecord for DNSLink
func txtRecordInput(name string, content string) cloudflare.DNSRecord {
	return cloudflare.DNSRecord{
		Type:    "TXT",
		Name:    name,
		Content: content,
	}
}

// cnameRecordInput generates a cloudflare.DNSRecord for CNAME
func cnameRecordInput(name string, content string) cloudflare.DNSRecord {
	return cloudflare.DNSRecord{
		Type:    "CNAME",
		Name:    name,
		Content: content,
		Proxied: true,
	}
}
