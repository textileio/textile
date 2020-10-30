package cli

import (
	"context"
	"crypto/tls"
	"net/http"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	stripe "github.com/stripe/stripe-go/v72"
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
	Short: "Setup usage billing",
	Long:  `Sets up metered usage billing.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		configureStripe(c)

		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		err := clients.Hub.SetupBilling(ctx)
		cmd.ErrCheck(err)

		cmd.Success("You have setup metered usage billing. " +
			"Use `hub billing portal` to manage your subscripton and payment methods.")
	},
}

var billingPortalCmd = &cobra.Command{
	Use:   "portal",
	Short: "Open billing web portal",
	Long:  `Opens a web portal for managing billing preferences.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		configureStripe(c)

		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		session, err := clients.Hub.GetBillingSession(ctx)
		cmd.ErrCheck(err)

		cmd.Message("Please visit the following URL to manage usage billing:")
		cmd.Message("%s", aurora.White(session.Url).Bold())
	},
}

func configureStripe(c *cobra.Command) {
	api, err := c.Flags().GetString("stripeApiUrl")
	cmd.ErrCheck(err)
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	err = http2.ConfigureTransport(transport)
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
	key, err := c.Flags().GetString("stripeApiKey")
	cmd.ErrCheck(err)
	stripe.Key = key
}
