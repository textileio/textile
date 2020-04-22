package email

import (
	"bytes"
	"context"
	"fmt"
	"net/mail"
	"text/template"

	logging "github.com/ipfs/go-log"
	mailgun "github.com/mailgun/mailgun-go/v3"
	"github.com/textileio/go-threads/util"
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
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"email": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	if _, err := mail.ParseAddress(from); err != nil {
		log.Fatalf("error parsing from email address: %v", err)
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

type confirmData struct {
	Link string
}

// ConfirmAddress sends a confirmation link to a recipient.
func (e *Client) ConfirmAddress(ctx context.Context, to, url, secret string) error {
	var tpl bytes.Buffer
	if err := e.verificationTmp.Execute(&tpl, &confirmData{
		Link: fmt.Sprintf("%s/confirm/%s", url, secret),
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
func (e *Client) InviteAddress(ctx context.Context, team, from, to, url, token string) error {
	var tpl bytes.Buffer
	if err := e.inviteTmp.Execute(&tpl, &inviteData{
		From: from,
		Team: team,
		Link: fmt.Sprintf("%s/consent/%s", url, token),
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
