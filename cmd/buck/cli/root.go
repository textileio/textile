package cli

import (
	"path/filepath"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/textile/util"
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
		key := config.Viper.GetString("key")

		buck, err := local.NewBucket(root, options.BalancedLayout)
		if err != nil {
			cmd.Fatal(err)
		}
		rc := buck.Remote()
		if !rc.Defined() {
			rc = getRemoteRoot(key)
		}
		cmd.Message("%s", aurora.White(rc).Bold())
	},
}

func getRemoteRoot(key string) cid.Cid {
	ctx, cancel := clients.Ctx.Thread(cmd.Timeout)
	defer cancel()
	rr, err := clients.Buckets.Root(ctx, key)
	if err != nil {
		cmd.Fatal(err)
	}
	rp, err := util.NewResolvedPath(rr.Root.Path)
	if err != nil {
		cmd.Fatal(err)
	}
	return rp.Cid()
}
