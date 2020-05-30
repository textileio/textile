package cli

import (
	"github.com/spf13/cobra"
)

var bucketCmd = &cobra.Command{
	Use: "buck",
	Aliases: []string{
		"bucket",
	},
	Short: "Manage an object storage bucket",
	Long:  `Manages files and folders in an object storage bucket.`,
	Args:  cobra.ExactArgs(0),
}
