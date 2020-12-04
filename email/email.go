package email

import (
	"context"
	"fmt"

	"github.com/customerio/go-customerio"
	logging "github.com/ipfs/go-log"
	"github.com/textileio/go-threads/util"
)

var (
	log = logging.Logger("email")
)

// Email service.
type Email struct {
	inviteTmpl  string
	confirmTmpl string
	client      *customerio.APIClient
}

// NewClient return a email api client.
func NewClient(confirmTmpl, inviteTmpl, apiKey string, debug bool) (*Email, error) {
	if debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"email": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	var client *customerio.APIClient
	if apiKey != "" {
		client = customerio.NewAPIClient(apiKey)
	}

	api := &Email{
		inviteTmpl:  inviteTmpl,
		confirmTmpl: confirmTmpl,
		client:      client,
	}

	return api, nil
}

// ConfirmAddress sends a confirmation link to a recipient.
func (sg *Email) ConfirmAddress(ctx context.Context, id, username, email, url, secret string) error {
	if sg.client == nil {
		log.Debug("Skipping email send")
		return nil
	}
	request := customerio.SendEmailRequest{
		To:                     email,
		TransactionalMessageID: sg.confirmTmpl,
		Identifiers: map[string]string{
			"id":           id,
			"confirmation": "true",
		},
		MessageData: map[string]interface{}{
			"link":     fmt.Sprintf("%s/confirm/%s", url, secret),
			"username": username,
		},
	}

	_, err := sg.client.SendEmail(ctx, &request)
	if err != nil {
		log.Fatal(err)
	}
	return err
}

// InviteAddress sends a confirmation link to a recipient.
func (sg *Email) InviteAddress(ctx context.Context, id, org, email, to, url, token string) error {
	if sg.client == nil {
		log.Debug("Skipping email send")
		return nil
	}
	request := customerio.SendEmailRequest{
		To:                     to,
		TransactionalMessageID: sg.inviteTmpl,
		Identifiers: map[string]string{
			"id":    id,
			"email": email,
		},
		MessageData: map[string]interface{}{
			"link": fmt.Sprintf("%s/consent/%s", url, token),
			"org":  org,
			"from": email,
		},
	}

	_, err := sg.client.SendEmail(ctx, &request)
	if err != nil {
		log.Fatal(err)
	}
	return err
}
