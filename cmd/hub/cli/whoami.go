package cli

import (
	"context"

	mbase "github.com/multiformats/go-multibase"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/cmd"
)

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user",
	Long:  `Shows the user for the current session.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		who, err := clients.Hub.GetSessionInfo(ctx)
		cmd.ErrCheck(err)
		key, err := mbase.Encode(mbase.Base32, who.Key)
		cmd.ErrCheck(err)
		cmd.Message("You are %s", aurora.White(who.Username).Bold())
		cmd.Message("Your key is %s", aurora.White(key).Bold())
	},
}
