package cli

import (
	"fmt"
	"os"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy your account",
	Long:  `Destroys your Hub account and all associated data.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := clients.Ctx.Auth(cmd.Timeout)
		defer cancel()
		who, err := clients.Hub.GetSessionInfo(ctx)
		if err != nil {
			cmd.Fatal(err)
		}

		cmd.Warn("%s", aurora.Red("Are you absolutely sure? This action cannot be undone. Your account and all associated data will be permanently deleted."))
		prompt := promptui.Prompt{
			Label: fmt.Sprintf("Please type '%s' to confirm", who.Username),
			Validate: func(s string) error {
				if s != who.Username {
					return fmt.Errorf("")
				}
				return nil
			},
		}
		if _, err := prompt.Run(); err != nil {
			cmd.End("")
		}

		ctx2, cancel2 := clients.Ctx.Auth(cmd.Timeout)
		defer cancel2()
		if err := clients.Hub.DestroyAccount(ctx2); err != nil {
			cmd.Fatal(err)
		}
		_ = os.RemoveAll(config.Viper.ConfigFileUsed())
		cmd.Success("Your account has been deleted")
	},
}
