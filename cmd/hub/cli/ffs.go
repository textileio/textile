package cli

import (
	"context"
	"encoding/json"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

var ffsCmd = &cobra.Command{
	Use:   "ffsInfo",
	Short: "Do something with ffs.",
	Long:  `Do something with ffs.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		info, err := clients.Powergate.FFS.Info(ctx)
		cmd.ErrCheck(err)
		bytes, err := json.MarshalIndent(info, "", "    ")
		cmd.ErrCheck(err)
		cmd.Message("FFS instance info:\n%v", string(bytes))
	},
}
