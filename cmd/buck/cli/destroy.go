package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	bucks "github.com/textileio/textile/buckets"
	"github.com/textileio/textile/cmd"
)

var bucketDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy bucket and all objects",
	Long:  `Destroys the bucket and all objects.`,
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

		cmd.Warn("%s", aurora.Red("This action cannot be undone. The bucket and all associated data will be permanently deleted."))
		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Are you absolutely sure"),
			IsConfirm: true,
		}
		if _, err := prompt.Run(); err != nil {
			cmd.End("")
		}

		ctx, cancel := clients.Ctx.Thread(cmd.Timeout)
		defer cancel()
		key := config.Viper.GetString("key")
		if err := clients.Buckets.Remove(ctx, key); err != nil {
			cmd.Fatal(err)
		}

		_ = os.RemoveAll(filepath.Join(root, bucks.SeedName))
		_ = os.RemoveAll(filepath.Join(root, config.Dir))
		cmd.Success("Your bucket has been deleted")
	},
}
