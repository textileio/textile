package email

import (
	"context"
	"encoding/json"
	"fmt"
	"net/mail"

	logging "github.com/ipfs/go-log"
	"github.com/sendgrid/sendgrid-go"
	"github.com/textileio/go-threads/util"
)

var (
	log = logging.Logger("email")
)

// Email service.
type Email struct {
	apiKey      string
	from        string
	inviteTmpl  string
	confirmTmpl string
	host        string
	path        string
	debug       bool
}

// NewClient return a email api client.
func NewClient(from string, confirmTmpl string, inviteTmpl string, apiKey string, debug bool) (*Email, error) {
	if debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"email": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	if _, err := mail.ParseAddress(from); err != nil {
		log.Fatalf("error parsing from email address: %v", err)
	}

	client := &Email{
		apiKey:      apiKey,
		from:        from,
		inviteTmpl:  inviteTmpl,
		confirmTmpl: confirmTmpl,
		host:        "https://api.sendgrid.com",
		path:        "/v3/mail/send",
		debug:       debug,
	}

	return client, nil
}

func (sg *Email) getBody(template_id, to_email string, group_id int, template_data map[string]string) ([]byte, error) {
	template, err := json.Marshal(template_data)
	if err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(` {
		"template_id":"%s",
		"from": {
			"email": "%s", 
			"name": "Textile Hub"
		},
		"asm": {
			"group_id": %d,
		},
		"personalizations": [
			{
				"to": [
					{
						"email": "%s"
					}
				],
				"from": {
					"email": "%s", 
					"name": "Textile Hub"
				},
				"dynamic_template_data": %s,
			},
		]
	}`, template_id, sg.from, group_id, to_email, sg.from, string(template))), nil
}

// ConfirmAddress sends a confirmation link to a recipient.
func (sg *Email) ConfirmAddress(ctx context.Context, username, email, url, secret string) error {
	link := fmt.Sprintf("%s/confirm/%s", url, secret)
	request := sendgrid.GetRequest(sg.apiKey, sg.path, sg.host)
	request.Method = "POST"
	body, err := sg.getBody(sg.confirmTmpl, email, 14540, map[string]string{"link": link, "username": username})
	if err != nil {
		return err
	}
	request.Body = body

	_, err = sendgrid.API(request)
	return err
}

// ConfirmAddress sends a confirmation link to a recipient.
func (sg *Email) InviteAddress(ctx context.Context, org, email, to, url, secret string) error {
	link := fmt.Sprintf("%s/confirm/%s", url, secret)
	request := sendgrid.GetRequest(sg.apiKey, sg.path, sg.host)
	request.Method = "POST"

	body, err := sg.getBody(sg.inviteTmpl, to, 14541, map[string]string{"link": link, "org": org, "from": email})
	if err != nil {
		return err
	}
	request.Body = body

	_, err = sendgrid.API(request)
	return err
}
