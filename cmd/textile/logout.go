package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
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
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.Logout(ctx, api.Auth{
			Token: authViper.GetString("token"),
		}); err != nil {
			cmd.Fatal(err)
		}

		_ = os.RemoveAll(authViper.ConfigFileUsed())

		cmd.Success("Bye :)")
	},
}
