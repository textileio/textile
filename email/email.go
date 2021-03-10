package email

import (
	"context"
	"fmt"

	cio "github.com/customerio/go-customerio"
	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/util"
)

var log = logging.Logger("email")

type Client struct {
	inviteTmpl  string
	confirmTmpl string
	client      *cio.APIClient
}

type Config struct {
	ConfirmTmpl string
	InviteTmpl  string
	APIKey      string
	Debug       bool
}

func NewClient(conf Config) (*Client, error) {
	if conf.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"email": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	var client *cio.APIClient
	if conf.APIKey != "" {
		client = cio.NewAPIClient(conf.APIKey)
	}
	return &Client{
		inviteTmpl:  conf.InviteTmpl,
		confirmTmpl: conf.ConfirmTmpl,
		client:      client,
	}, nil
}

// ConfirmAddress sends a confirmation link to a recipient.
func (c *Client) ConfirmAddress(ctx context.Context, id, username, email, url, secret string) error {
	if c.client == nil {
		return nil
	}
	request := cio.SendEmailRequest{
		To:                     email,
		TransactionalMessageID: c.confirmTmpl,
		Identifiers: map[string]string{
			"id":           id,
			"confirmation": "true",
		},
		MessageData: map[string]interface{}{
			"link":     fmt.Sprintf("%s/confirm/%s", url, secret),
			"username": username,
		},
	}
	if _, err := c.client.SendEmail(ctx, &request); err != nil {
		return err
	}

	log.Debugf("sent confirm address for %s to %s", username, email)
	return nil
}

// InviteAddress sends a confirmation link to a recipient.
func (c *Client) InviteAddress(ctx context.Context, id, org, from, to, url, token string) error {
	if c.client == nil {
		return nil
	}
	request := cio.SendEmailRequest{
		To:                     to,
		TransactionalMessageID: c.inviteTmpl,
		Identifiers: map[string]string{
			"id":    id,
			"email": from,
		},
		MessageData: map[string]interface{}{
			"link": fmt.Sprintf("%s/consent/%s", url, token),
			"org":  org,
			"from": from,
		},
	}
	if _, err := c.client.SendEmail(ctx, &request); err != nil {
		return err
	}

	log.Debug("sent invite to %s from %s to %s", org, from, to)
	return nil
}
