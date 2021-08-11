package analytics

import (
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/util"
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
}

// NewClient return a segment client.
func NewClient(segmentAPIKey, prefix string, debug bool) (*Client, error) {
	if debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"analytics": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	var api segment.Client
	if segmentAPIKey != "" {
		config := segment.Config{
			Verbose: debug,
		}
		var err error
		api, err = segment.NewWithConfig(segmentAPIKey, config)
		if err != nil {
			return nil, err
		}
	}

	return &Client{
		api:    api,
		prefix: prefix,
	}, nil
}

// Identify creates or updates the user traits
func (c *Client) Identify(key string, accountType mdb.AccountType, active bool, email string, properties map[string]interface{}) {
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
		if err := c.api.Enqueue(segment.Identify{
			UserId: key,
			Traits: traits,
			Context: &segment.Context{
				Extra: map[string]interface{}{
					"active": active,
				},
			},
		}); err != nil {
			log.Errorf("identifying user: %v", err)
		}
	}
}

// TrackEvent logs a new event
func (c *Client) TrackEvent(key string, accountType mdb.AccountType, active bool, event Event, properties map[string]string) {
	if c.api != nil && accountType != mdb.User {
		props := segment.NewProperties()
		for key, value := range properties {
			props.Set(key, value)
		}

		if err := c.api.Enqueue(segment.Track{
			UserId:     key,
			Event:      event.String(),
			Properties: props,
			Context: &segment.Context{
				Extra: map[string]interface{}{
					"active": active,
				},
			},
		}); err != nil {
			log.Errorf("tracking event: %v", err)
		}
	}
}

// FormatUnix converts seconds to string in same format for all analytics requests
func (c *Client) FormatUnix(seconds int64) string {
	return time.Unix(seconds, 0).Format(time.RFC3339)
}
