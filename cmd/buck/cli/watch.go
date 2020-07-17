package cli

import (
	"context"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/uiprogress"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch auto-pushes local changes to the remote",
	Long:  `Watch auto-pushes local changes to the remote.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, ".")
		cmd.ErrCheck(err)
		bp, err := buck.Path()
		cmd.ErrCheck(err)
		events := make(chan local.PathEvent)
		defer close(events)
		progress := uiprogress.New()
		progress.Start()
		go handleProgressBars(progress, events)
		state := buck.Watch(ctx, local.WithWatchEvents(events))
		for s := range state {
			switch s.State {
			case local.Online:
				cmd.Success("Watching %s for changes...", aurora.White(bp).Bold())
				if progress == nil {
					progress = uiprogress.New()
				}
				progress.Start()
			case local.Offline:
				if progress != nil {
					progress.Stop()
					progress = nil
				}
				if s.Fatal {
					cmd.Fatal(s.Err)
				} else {
					cmd.Message("Not connected. Trying to connect...")
				}
			}
		}
		cmd.ErrCheck(err)
	},
}
