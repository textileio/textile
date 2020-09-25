package cli

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/cmd"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout",
	Long:  `Handles logout of a Hub account.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		err := clients.Hub.Signout(ctx)
		cmd.ErrCheck(err)
		_ = os.RemoveAll(config.Viper.ConfigFileUsed())
		cmd.Success("Bye :)")
	},
}
