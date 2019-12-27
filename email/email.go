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

// Client wraps a MailGun client.
type Client struct {
	from            string
	gun             *mailgun.MailgunImpl
	verificationTmp *template.Template
	inviteTmp       *template.Template
	debug           bool
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
	it, err := template.New("invite").Parse(inviteMsg)
	if err != nil {
		log.Fatal(err)
	}

	client := &Client{
		from:            from,
		verificationTmp: vt,
		inviteTmp:       it,
		debug:           debug,
	}

	if apiKey != "" {
		client.gun = mailgun.NewMailgun(domain, apiKey)
	}
	return client, nil
}

type verificationData struct {
	Link string
}

// VerifyAddress sends a verification link to a recipient.
func (e *Client) VerifyAddress(ctx context.Context, to, link string) error {
	var tpl bytes.Buffer
	if err := e.verificationTmp.Execute(&tpl, &verificationData{
		Link: link,
	}); err != nil {
		return err
	}

	return e.send(ctx, to, "Textile Login Verification", tpl.String())
}

type inviteData struct {
	From string
	Team string
	Link string
}

// InviteAddress sends an invite link to a recipient.
func (e *Client) InviteAddress(ctx context.Context, team, from, to, link string) error {
	var tpl bytes.Buffer
	if err := e.verificationTmp.Execute(&tpl, &inviteData{
		From: from,
		Team: team,
		Link: link,
	}); err != nil {
		return err
	}

	return e.send(ctx, to, "Textile Team Invitation", tpl.String())
}

// send wraps the MailGun client's send method.
func (e *Client) send(ctx context.Context, recipient, subject, body string) error {
	if e.gun == nil {
		return nil
	}
	_, _, err := e.gun.Send(ctx, e.gun.NewMessage(e.from, subject, body, recipient))
	return err
}
