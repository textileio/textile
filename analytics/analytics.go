package analytics

import (
	logging "github.com/ipfs/go-log"
	"gopkg.in/segmentio/analytics-go.v3"
	segment "gopkg.in/segmentio/analytics-go.v3"
)

var (
	log = logging.Logger("analytics")
)

// Client uses segment to trigger life-cycle emails (quota, billing, etc).
type Client struct {
	api   segment.Client
	debug bool
}

// NewClient return a segment client.
func NewClient(segmentAPIKey string, debug bool) (*Client, error) {
	var api segment.Client
	var err error
	if segmentAPIKey != "" {
		config := segment.Config{
			Verbose: debug,
		}
		api, err = segment.NewWithConfig(segmentAPIKey, config)
	}

	client := &Client{
		api: api,
	}

	return client, err
}

// NewUser sets up a user for lifecycle events
func (c *Client) NewUser(userId string, email string, properties map[string]string) {
	if c.api != nil {
		traits := analytics.NewTraits()
		for key, value := range properties {
			traits.Set(key, value)
		}
		if email != "" {
			traits.SetEmail(email)
		}
		if err := c.api.Enqueue(segment.Identify{
			UserId: userId,
			Traits: traits,
		}); err != nil {
			log.Error("segmenting new customer: %v", err)
		}
	}
}

// NewEvent logs a new event
func (c *Client) NewEvent(userId string, eventName string, properties map[string]string) {
	if c.api != nil {
		props := analytics.NewProperties()
		for key, value := range properties {
			props.Set(key, value)
		}

		if err := c.api.Enqueue(segment.Track{
			UserId:     userId,
			Event:      eventName,
			Properties: props,
		}); err != nil {
			log.Error("segmenting new event: %v", err)
		}
	}
}
