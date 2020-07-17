package cli

import (
	"context"
	"fmt"

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
		state, err := buck.Watch(ctx, local.WithWatchEvents(events), local.WithOffline(true))
		cmd.ErrCheck(err)
		for s := range state {
			switch s.State {
			case local.Online:
				addInfoBar(progress, fmt.Sprintf("> Connected! Watching %s for changes...", bp))
			case local.Offline:
				if s.Fatal {
					cmd.Fatal(s.Err)
				} else {
					addInfoBar(progress, "> Not connected. Trying to connect...")
				}
			}
		}
		progress.Stop()
	},
}
