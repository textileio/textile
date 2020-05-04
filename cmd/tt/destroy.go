package main

import (
	"fmt"
	"os"

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
	Long:  `Destroy your account and all associated data.`,
	Run: func(c *cobra.Command, args []string) {
		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Delete your entire account and all associated data?"),
			IsConfirm: true,
		}
		if _, err := prompt.Run(); err != nil {
			cmd.End("")
		}

		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		if err := hub.DestroyAccount(ctx); err != nil {
			cmd.Fatal(err)
		}
		_ = os.RemoveAll(authViper.ConfigFileUsed())
		cmd.Success("Your account and all associated data has been completely destroyed")
	},
}
