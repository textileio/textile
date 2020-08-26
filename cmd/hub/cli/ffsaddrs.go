package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

var ffsAddrsCmd = &cobra.Command{
	Use:   "ffs-addrs",
	Short: "Do something with ffs.",
	Long:  `Do something with ffs.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()

		addrs, err := clients.Powergate.FFS.Addrs(ctx)
		cmd.ErrCheck(err)
		cmd.Message("FFS addrs:\n%v", addrs)
	},
}
