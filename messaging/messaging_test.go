package messaging

import (
	"testing"
)

func TestVerifyEmail(t *testing.T) {
	notifier := EmailService{
		"Textile <verify@email.textile.io>",
		"email.textile.io",
		"secret",
	}
	t.Skip("skipping test in dev.")
	t.Run("test send", func(t *testing.T) {
		err := notifier.VerifyAddress("somebody@new.new", "https://textile.io")
		if err == nil {
			t.Fatalf("email should fail with bad private key")
		}
	})
}
