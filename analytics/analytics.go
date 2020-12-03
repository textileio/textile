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
func NewClient(segmentApiKey, prefix string, debug bool) (*Client, error) {
	var api segment.Client
	var err error
	if segmentApiKey != "" {
		config := segment.Config{
			Verbose: debug,
		}
		api, err = segment.NewWithConfig(segmentApiKey, config)
	}

	client := &Client{
		api:    api,
		prefix: prefix,
		debug:  debug,
	}

	return client, err
}

// NewUpdate updates the user metadata
func (c *Client) NewUpdate(userId, email string, properties map[string]interface{}) {
	if c.api != nil {
		traits := analytics.NewTraits()
		for key, value := range properties {
			traits.Set(key, value)
		}
		traits.Set(c.prefix+"signup", "true")
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
func (c *Client) NewEvent(userId, eventName string, properties map[string]interface{}) {
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

func (c *Client) FormatTime(nanos int64) string {
	return time.Unix(0, nanos).Format(time.RFC3339)
}
