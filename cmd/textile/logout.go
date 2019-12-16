package main

import (
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(logoutCmd)
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Logout",
	Long:  `Logout of Textile.`,
	Run: func(c *cobra.Command, args []string) {
		_ = os.RemoveAll(authViper.ConfigFileUsed())
	},
}
