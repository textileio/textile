package cli

import (
	"context"
	"errors"
	"strings"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/buckets"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
)

const nonFastForwardMsg = "the root of your bucket is behind (try `%s` before pushing again)"

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push bucket object changes",
	Long: `Pushes paths that have been added to and paths that have been removed or differ from the local bucket root.

Use the '--force' flag to allow a non-fast-forward update.
`,
	Args: cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		force, err := c.Flags().GetBool("force")
		cmd.ErrCheck(err)
		yes, err := c.Flags().GetBool("yes")
		cmd.ErrCheck(err)
		quiet, err := c.Flags().GetBool("quiet")
		cmd.ErrCheck(err)
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.PushTimeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)

		var events chan local.Event
		if !quiet {
			events = make(chan local.Event)
			defer close(events)
			go handleEvents(events)
		}
		roots, err := buck.PushLocal(
			ctx,
			local.WithConfirm(getConfirm("Push %d changes", yes)),
			local.WithForce(force),
			local.WithEvents(events),
		)
		if errors.Is(err, local.ErrAborted) {
			cmd.End("")
		} else if errors.Is(err, local.ErrUpToDate) {
			cmd.End("Everything up-to-date")
		} else if err != nil && strings.Contains(err.Error(), buckets.ErrNonFastForward.Error()) {
			cmd.Fatal(errors.New(nonFastForwardMsg), aurora.Cyan("buck pull"))
		} else if err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("%s", aurora.White(roots.Remote).Bold())
	},
}
