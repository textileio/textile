package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

var bucketCatCmd = &cobra.Command{
	Use:   "cat [path]",
	Short: "Cat bucket objects at path",
	Long:  `Cats bucket objects at path.`,
	Args:  cobra.ExactArgs(1),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(config.Viper, config.Flags)
		if config.Viper.ConfigFileUsed() == "" {
			cmd.Fatal(errNotABucket)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := clients.Ctx.Thread(getFileTimeout)
		defer cancel()
		key := config.Viper.GetString("key")
		if err := clients.Buckets.PullPath(ctx, key, args[0], os.Stdout); err != nil {
			cmd.Fatal(err)
		}
	},
}
