package dns

import (
	"fmt"

	"github.com/cloudflare/cloudflare-go"
	logging "github.com/ipfs/go-log"
	"github.com/textileio/go-threads/util"
)

var (
	log = logging.Logger("dns")
)

// Client wraps a CloudflareClient client.
type Client struct {
	api    *cloudflare.API
	zoneID string
	debug  bool
}

// NewClient return a cloudflare-backed dns updating client.
func NewClient(domain string, email string, zoneID string, apiKey string, debug bool) (*Client, error) {
	if debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"dns": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	client := &Client{
		debug: debug,
	}

	if apiKey != "" {
		api, err := cloudflare.New(apiKey, email)
		if err != nil {
			log.Fatal(err)
		}
		client.api = api
		client.zoneID = zoneID
	}
	return client, nil
}

func (c *Client) Create(subdomain string) {

	cname := cloudflare.DNSRecord{
		Type:    "CNAME",
		Name:    subdomain,
		Content: "www.cloudflare-ipfs.com",
	}
	// parse err here
	_, err := c.api.CreateDNSRecord(c.zoneID, cname)
	if err != nil {
		fmt.Println(err)
		return
	}
	dnslink := cloudflare.DNSRecord{
		Type:    "TXT",
		Name:    fmt.Sprintf("_dnslink.%s", subdomain),
		Content: "dnslink=/ipfs/Qmf65nLokMjpkQjzgs68ik5AuHSu7NVSjSiGtko5ABp6FY",
	}
	_, err = c.api.CreateDNSRecord(c.zoneID, dnslink)
	if err != nil {
		fmt.Println(err)
	}
}
