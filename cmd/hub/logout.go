package main

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(logoutCmd)
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout",
	Long:  `Handles logout of a Hub account.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := clients.Ctx.Auth(cmd.Timeout)
		defer cancel()
		if err := clients.Hub.Signout(ctx); err != nil {
			cmd.Fatal(err)
		}
		_ = os.RemoveAll(config.Viper.ConfigFileUsed())
		cmd.Success("Bye :)")
	},
}
