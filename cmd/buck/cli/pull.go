package cli

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
)

var pullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull bucket object changes",
	Long: `Pulls paths that have been added to and paths that have been removed or differ from the remote bucket root.

Use the '--hard' flag to discard all local changes.
Use the '--force' flag to pull all remote objects, even if they already exist locally.
`,
	Args: cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		force, err := c.Flags().GetBool("force")
		cmd.ErrCheck(err)
		hard, err := c.Flags().GetBool("hard")
		cmd.ErrCheck(err)
		yes, err := c.Flags().GetBool("yes")
		cmd.ErrCheck(err)
		quiet, err := c.Flags().GetBool("quiet")
		cmd.ErrCheck(err)
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.PullTimeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		var events chan local.Event
		if !quiet {
			events = make(chan local.Event)
			defer close(events)
			go handleEvents(events)
		}
		roots, err := buck.PullRemote(
			ctx,
			local.WithConfirm(getConfirm("Discard %d local changes", yes)),
			local.WithForce(force),
			local.WithHard(hard),
			local.WithEvents(events))
		if errors.Is(err, local.ErrAborted) {
			cmd.End("")
		} else if errors.Is(err, local.ErrUpToDate) {
			cmd.End("Everything up-to-date")
		} else if err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("%s", aurora.White(roots.Remote).Bold())
	},
}
