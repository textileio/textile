package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/buckets"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/uiprogress"
)

var (
	buckMaxSizeMiB = int64(1024)
	MiB            = int64(1024 * 1024)
)

const nonFastForwardMsg = "the root of your bucket is behind (try `%s` before pushing again)"

var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push bucket object changes",
	Long:  `Pushes paths that have been added to and paths that have been removed or differ from the local bucket root.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		force, err := c.Flags().GetBool("force")
		cmd.ErrCheck(err)
		yes, err := c.Flags().GetBool("yes")
		cmd.ErrCheck(err)
		maxSize, err := c.Flags().GetInt64("maxsize")
		if err != nil {
			cmd.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), cmd.PushTimeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, ".")
		cmd.ErrCheck(err)

		// Check total bucket size limit.
		size, err := buck.LocalSize()
		cmd.ErrCheck(err)
		if size > maxSize*MiB {
			cmd.Fatal(fmt.Errorf("the bucket size is %dMB which is bigger than accepted limit %dMB", size/MiB, maxSize))
		}

		events := make(chan local.PathEvent)
		defer close(events)
		progress := uiprogress.New()
		progress.Start()
		go handleProgressBars(progress, events)
		roots, err := buck.PushLocal(
			ctx,
			local.WithConfirm(getConfirm("Push %d changes", yes)),
			local.WithForce(force),
			local.WithPathEvents(events))
		progress.Stop()
		if errors.Is(err, local.ErrAborted) {
			cmd.End("")
		} else if errors.Is(err, local.ErrUpToDate) {
			cmd.End("Everything up-to-date")
		} else if errors.Is(err, buckets.ErrNonFastForward) {
			cmd.Fatal(errors.New(nonFastForwardMsg), aurora.Cyan("buck pull"))
		} else if err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("%s", aurora.White(roots.Remote).Bold())
	},
}
