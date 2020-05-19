package main

import (
	"fmt"
	"os"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(destroyCmd)
}

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy your account",
	Long:  `Destroy your Hub account and all associated data.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		who, err := hub.GetSessionInfo(ctx)
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

		ctx2, cancel2 := authCtx(cmdTimeout)
		defer cancel2()
		if err := hub.DestroyAccount(ctx2); err != nil {
			cmd.Fatal(err)
		}
		_ = os.RemoveAll(authViper.ConfigFileUsed())
		cmd.Success("Your account has been deleted")
	},
}
