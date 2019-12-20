package email

import (
	"bytes"
	"context"
	"text/template"

	logging "github.com/ipfs/go-log"
	mailgun "github.com/mailgun/mailgun-go/v3"
	"github.com/textileio/go-threads/util"
	logger "github.com/whyrusleeping/go-logging"
)

var (
	log = logging.Logger("email")
)

const (
	verificationMsg = `To confirm your email click here: {{.Link}}`
)

// Client holds MailGun account details.
type Client struct {
	from            string
	gun             *mailgun.MailgunImpl
	verificationTmp *template.Template
	debug           bool
}

// messageValue holds message data.
type messageValue struct {
	Link string
}

// NewClient return a mailgun-backed email client.
func NewClient(from, domain, apiKey string, debug bool) (*Client, error) {
	if debug {
		if err := util.SetLogLevels(map[string]logger.Level{
			"email": logger.DEBUG,
		}); err != nil {
			return nil, err
		}
	}

	vt, err := template.New("verification").Parse(verificationMsg)
	if err != nil {
		log.Fatal(err)
	}

	client := &Client{
		from:            from,
		verificationTmp: vt,
		debug:           debug,
	}

	if apiKey != "" {
		client.gun = mailgun.NewMailgun(domain, apiKey)
	}
	return client, nil
}

// VerifyAddress sends a verification link to a recipient.
func (e *Client) VerifyAddress(ctx context.Context, recipient, link string) error {
	var tpl bytes.Buffer
	if err := e.verificationTmp.Execute(&tpl, &messageValue{
		Link: link,
	}); err != nil {
		return err
	}

	// Treat as dummy service if the underlying client doesn't exist.
	if e.gun == nil {
		return nil
	}
	return e.send(ctx, recipient, "Email Confirmation", tpl.String())
}

func (e *Client) send(ctx context.Context, recipient, subject, body string) error {
	_, _, err := e.gun.Send(ctx, e.gun.NewMessage(e.from, subject, body, recipient))
	return err
}
