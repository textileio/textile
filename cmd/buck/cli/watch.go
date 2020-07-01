package cli

import (
	"context"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch auto-pushes local changes to the remote",
	Long:  `Watch auto-pushes local changes to the remote.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		interval, err := c.Flags().GetDuration("interval")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, ".")
		cmd.ErrCheck(err)
		bp, err := buck.Path()
		cmd.ErrCheck(err)
		events := make(chan local.PathEvent)
		defer close(events)
		go handleProgressBars(events, true)
		cmd.Message("Watching bucket in %s for changes...", aurora.White(bp).Bold())
		err = buck.Watch(ctx, local.WithInterval(interval), local.WithWatchEvents(events))
		cmd.ErrCheck(err)
	},
}
