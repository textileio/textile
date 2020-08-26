package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

var ffsIDCmd = &cobra.Command{
	Use:   "ffs-id",
	Short: "Do something with ffs.",
	Long:  `Do something with ffs.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()

		id, err := clients.Powergate.FFS.ID(ctx)
		cmd.ErrCheck(err)
		cmd.Message("FFS ID: %v", id)
	},
}
