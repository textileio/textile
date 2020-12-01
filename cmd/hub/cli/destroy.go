package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/cmd"
)

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy your account",
	Long:  `Destroys your Hub account and all associated data.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		who, err := clients.Hub.GetSessionInfo(ctx)
		cmd.ErrCheck(err)

		cmd.Warn("%s", aurora.Red("Are you absolutely sure? This action cannot be undone."))
		cmd.Warn("%s", aurora.Red("Your account and all associated data will be permanently deleted."))
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

		err = clients.Hub.DestroyAccount(ctx)
		cmd.ErrCheck(err)
		_ = os.RemoveAll(config.Viper.ConfigFileUsed())
		cmd.Success("Your account has been deleted")
	},
}
