package cli

import (
	"path/filepath"

	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
)

var bucketRootCmd = &cobra.Command{
	Use:   "root",
	Short: "Show local bucket root CID",
	Long:  `Shows the local bucket root CID`,
	Args:  cobra.ExactArgs(0),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(config.Viper, config.Flags)
	},
	Run: func(c *cobra.Command, args []string) {
		conf := config.Viper.ConfigFileUsed()
		if conf == "" {
			cmd.Fatal(errNotABucket)
		}
		root := filepath.Dir(filepath.Dir(conf))
		buck, err := local.NewBucket(root, options.BalancedLayout)
		if err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("%s", aurora.White(buck.Path().Cid()).Bold())
	},
}
