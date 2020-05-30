package cli

import (
	"github.com/logrusorgru/aurora"
	mbase "github.com/multiformats/go-multibase"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user",
	Long:  `Shows the user for the current session.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := clients.Ctx.Auth(cmd.Timeout)
		defer cancel()
		who, err := clients.Hub.GetSessionInfo(ctx)
		if err != nil {
			cmd.Fatal(err)
		}
		key, err := mbase.Encode(mbase.Base32, who.Key)
		if err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("You are %s", aurora.White(who.Username).Bold())
		cmd.Message("Your key is %s", aurora.White(key).Bold())
	},
}
