package cli

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

var netPeersCmd = &cobra.Command{
	Use:   "net-peers",
	Short: "Do something with net.",
	Long:  `Do something with net.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()

		peers, err := clients.Pow.Peers(ctx)
		cmd.ErrCheck(err)
		cmd.Message("PEERS: %v", peers)
	},
}
