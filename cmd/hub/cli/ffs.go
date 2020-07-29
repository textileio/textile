package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

var ffsCmd = &cobra.Command{
	Use:   "ffs",
	Short: "Do something with ffs.",
	Long:  `Do something with ffs.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		id, token, err := clients.Powergate.FFS.Create(ctx)
		cmd.ErrCheck(err)
		cmd.Message("FFS ID %v and token %v", id, token)
	},
}
