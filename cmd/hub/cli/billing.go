package cli

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	stripe "github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/token"
	"github.com/textileio/textile/v2/cmd"
	"golang.org/x/net/http2"
)

var billingCmd = &cobra.Command{
	Use:   "billing",
	Short: "Billing management",
	Long:  `Manages your billing preferences.`,
	Args:  cobra.ExactArgs(0),
}

var billingSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup billing preferences",
	Long:  `Sets up billing preferences.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		api, err := c.Flags().GetString("stripeApiUrl")
		cmd.ErrCheck(err)
		configureStripe(api)
		key, err := c.Flags().GetString("stripeApiKey")
		cmd.ErrCheck(err)
		stripe.Key = key

		cmd.Message("By setting up billing you're opting into a recurring subscription. " +
			"You will be asked for credit card details.")
		prompt := promptui.Prompt{
			Label:     "Proceed",
			IsConfirm: true,
		}
		if _, err := prompt.Run(); err != nil {
			cmd.End("")
		}

		prompt = promptui.Prompt{
			Label: "Card number",
		}
		cardNumber, err := prompt.Run()
		if err != nil {
			cmd.End("")
		}
		prompt = promptui.Prompt{
			Label: "Card expiration month",
		}
		cardExpMonth, err := prompt.Run()
		if err != nil {
			cmd.End("")
		}
		prompt = promptui.Prompt{
			Label: "Card expiration year",
		}
		cardExpYear, err := prompt.Run()
		if err != nil {
			cmd.End("")
		}
		prompt = promptui.Prompt{
			Label: "Card CVC",
			Mask:  '*',
		}
		cardCVC, err := prompt.Run()
		if err != nil {
			cmd.End("")
		}
		tok, err := token.New(&stripe.TokenParams{
			Card: &stripe.CardParams{
				Number:   stripe.String(cardNumber),
				ExpMonth: stripe.String(cardExpMonth),
				ExpYear:  stripe.String(cardExpYear),
				CVC:      stripe.String(cardCVC),
			},
		})
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		err = clients.Hub.SetupBilling(ctx, tok.ID)
		cmd.ErrCheck(err)
		cmd.Success("You have setup metered usage billing. See <insert link> for more billing details.")
	},
}

func configureStripe(api string) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	err := http2.ConfigureTransport(transport)
	cmd.ErrCheck(err)
	stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(
		stripe.APIBackend,
		&stripe.BackendConfig{
			URL: stripe.String(api),
			HTTPClient: &http.Client{
				Transport: transport,
			},
			LeveledLogger: stripe.DefaultLeveledLogger,
		},
	))
}
