package messaging

import (
	"bytes"
	"context"
	"fmt"
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

	data := struct {
		Link string
	}{
		Link: link,
	}

	var tpl bytes.Buffer
	if err := t.Execute(&tpl, data); err != nil {
		return err
	}

	result := tpl.String()

	return n.sendWithMailgun(recipient, subject, result)
}

func (n *EmailService) sendWithMailgun(recipient string, subject string, body string) error {
	// Non-erroring skip if email service not configured
	if n.PrivateKey == "" {
		return nil
	}
	mg := mailgun.NewMailgun(n.Domain, n.PrivateKey)

	// The message object allows you to add attachments and Bcc recipients
	message := mg.NewMessage(n.From, subject, body, recipient)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Send the message	with a 10 second timeout
	resp, id, err := mg.Send(ctx, message)

	if err != nil {
		log.Error(err)
	}

	fmt.Printf("ID: %s Resp: %s\n", id, resp)

	return err
}
