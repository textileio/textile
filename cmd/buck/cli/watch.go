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
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, ".")
		cmd.ErrCheck(err)
		bp, err := buck.Path()
		cmd.ErrCheck(err)
		events := make(chan local.PathEvent)
		defer close(events)
		go handleWatchEvents(events)
		state, err := buck.Watch(ctx, local.WithWatchEvents(events), local.WithOffline(true))
		cmd.ErrCheck(err)
		for s := range state {
			switch s.State {
			case local.Online:
				cmd.Success("Watching %s for changes...", aurora.White(bp).Bold())
			case local.Offline:
				if s.Fatal {
					cmd.Fatal(s.Err)
				} else {
					cmd.Message("Not connected. Trying to connect...")
				}
			}
		}
	},
}

func handleWatchEvents(events chan local.PathEvent) {
	for e := range events {
		switch e.Type {
		case local.FileComplete:
			cmd.Message("%s: %s (%s)", aurora.Green("+ "+e.Path), e.Cid, formatBytes(e.Size, true))
		case local.FileRemoved:
			cmd.Message("%s", aurora.Red("- "+e.Path))
		}
	}
}
