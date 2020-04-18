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
	Long:  `Logout of Textile.`,
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		if err := hub.Signout(ctx); err != nil {
			cmd.Fatal(err)
		}
		_ = os.RemoveAll(authViper.ConfigFileUsed())
		cmd.Success("Bye :)")
	},
}
