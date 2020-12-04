package analytics

import (
	"time"

	logging "github.com/ipfs/go-log"
	"gopkg.in/segmentio/analytics-go.v3"
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

// Update updates the user metadata
func (c *Client) Update(userID, email string, active bool, properties map[string]interface{}) {
	if c.api != nil && email != "" {
		traits := analytics.NewTraits()
		traits.SetEmail(email)
		for key, value := range properties {
			traits.Set(key, value)
		}
		traits.Set(c.prefix+"signup", "true")
		if err := c.api.Enqueue(segment.Identify{
			UserId: userID,
			Traits: traits,
			Context: &analytics.Context{
				Extra: map[string]interface{}{
					"active": active,
				},
			},
		}); err != nil {
			log.Error("segmenting new customer: %v", err)
		}
	}
}

// NewEvent logs a new event
func (c *Client) NewEvent(userID, email, eventName string, active bool, properties map[string]interface{}) {
	if c.api != nil && email != "" {
		props := analytics.NewProperties()
		for key, value := range properties {
			props.Set(key, value)
		}

		if err := c.api.Enqueue(segment.Track{
			UserId:     userID,
			Event:      eventName,
			Properties: props,
			Context: &analytics.Context{
				Extra: map[string]interface{}{
					"active": active,
				},
			},
		}); err != nil {
			log.Error("segmenting new event: %v", err)
		}
	}
}

// FormatTime converts nanos to string in same format for all analytics requests
func (c *Client) FormatTime(nanos int64) string {
	return time.Unix(0, nanos).Format(time.RFC3339)
}
