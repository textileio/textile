package messaging

import (
	"bytes"
	"context"
	"text/template"
	"time"

	logging "github.com/ipfs/go-log"
	mailgun "github.com/mailgun/mailgun-go/v3"
)

var (
	log = logging.Logger("messaging")
)

// EmailService holds MailGun account details
type EmailService struct {
	From       string
	Domain     string
	PrivateKey string
}

type messageValue struct {
	Link string
}

// VerifyAddress sends a verification link to a receipient
func (n *EmailService) VerifyAddress(recipient string, link string) error {
	subject := "Email Confirmation"

	t := template.New("verification")

	const email = `To confirm your email click here: {{.Link}}`

	var err error
	t, err = t.Parse(email)
	if err != nil {
		return err
	}

	data := messageValue{
		Link: link,
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, data); err != nil {
		return err
	}

	result := tpl.String()

	// Treat as dummy service if no key set
	if n.PrivateKey == "" {
		return nil
	}
	return n.sendWithMailgun(recipient, subject, result)
}

func (n *EmailService) sendWithMailgun(recipient string, subject string, body string) error {
	if n.PrivateKey == "" {
		return nil
	}
	mg := mailgun.NewMailgun(n.Domain, n.PrivateKey)

	message := mg.NewMessage(n.From, subject, body, recipient)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	_, _, err := mg.Send(ctx, message)

	if err != nil {
		log.Error(err)
	}

	return err
}
