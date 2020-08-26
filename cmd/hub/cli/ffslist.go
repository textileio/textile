package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

var ffsListCmd = &cobra.Command{
	Use:   "ffs-list",
	Short: "Do something with ffs.",
	Long:  `Do something with ffs.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()

		list, err := clients.Powergate.FFS.ListAPI(ctx)
		cmd.ErrCheck(err)
		cmd.Message("FFS instances list: %v", list)
	},
}
