package analytics

import (
	"time"

	logging "github.com/ipfs/go-log/v2"
	mdb "github.com/textileio/textile/v2/mongodb"
	segment "gopkg.in/segmentio/analytics-go.v3"
)

var (
	log = logging.Logger("analytics")
)

// Client uses segment to trigger life-cycle emails (quota, billing, etc).
type Client struct {
	api    segment.Client
	prefix string
	debug  bool
}

// NewClient return a segment client.
func NewClient(segmentAPIKey, prefix string, debug bool) (*Client, error) {
	var api segment.Client
	var err error
	if segmentAPIKey != "" {
		config := segment.Config{
			Verbose: debug,
		}
		api, err = segment.NewWithConfig(segmentAPIKey, config)
	}

	client := &Client{
		api:    api,
		prefix: prefix,
		debug:  debug,
	}

	return client, err
}

// Identify creates or updates the user traits
func (c *Client) Identify(key string, accountType mdb.AccountType, active bool, email string, properties map[string]interface{}) error {
	if c.api != nil && accountType != mdb.User {
		traits := segment.NewTraits()
		traits.Set("account_type", accountType)
		traits.Set(c.prefix+"signup", "true")
		if email != "" {
			traits.SetEmail(email)
		}
		for key, value := range properties {
			traits.Set(key, value)
		}
		return c.api.Enqueue(segment.Identify{
			UserId: key,
			Traits: traits,
			Context: &segment.Context{
				Extra: map[string]interface{}{
					"active": active,
				},
			},
		})
	}
	return nil
}

// TrackEvent logs a new event
func (c *Client) TrackEvent(key string, accountType mdb.AccountType, active bool, event Event, properties map[string]string) error {
	if c.api != nil && accountType != mdb.User {
		props := segment.NewProperties()
		for key, value := range properties {
			props.Set(key, value)
		}

		return c.api.Enqueue(segment.Track{
			UserId:     key,
			Event:      event.String(),
			Properties: props,
			Context: &segment.Context{
				Extra: map[string]interface{}{
					"active": active,
				},
			},
		})
	}
	return nil
}

// FormatUnix converts seconds to string in same format for all analytics requests
func (c *Client) FormatUnix(seconds int64) string {
	return time.Unix(seconds, 0).Format(time.RFC3339)
}
