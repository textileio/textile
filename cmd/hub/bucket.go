package main

import (
	"github.com/spf13/cobra"
	buck "github.com/textileio/textile/cmd/buck/cmds"
)

func init() {
	rootCmd.AddCommand(bucketCmd)
	buck.InitCmd(bucketCmd)
}

var bucketCmd = &cobra.Command{
	Use: "bucket",
	Aliases: []string{
		"buck",
	},
	Short: "Manage an object storage bucket",
	Long:  `Manages files and folders in an object storage bucket.`,
	Args:  cobra.ExactArgs(0),
}
