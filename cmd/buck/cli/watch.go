package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
)

var watchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch auto-pushes local changes to the remote",
	Long:  `Watch auto-pushes local changes to the remote.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		bp, err := buck.Path()
		cmd.ErrCheck(err)
		events := make(chan local.Event)
		defer close(events)
		go handleWatchEvents(events)
		state, err := buck.Watch(ctx, local.WithWatchEvents(events), local.WithOffline(true))
		cmd.ErrCheck(err)
		for s := range state {
			switch s.State {
			case cmd.Online:
				cmd.Success("Watching %s for changes...", aurora.White(bp).Bold())
			case cmd.Offline:
				if s.Aborted {
					cmd.Fatal(s.Err)
				} else {
					cmd.Message("Not connected. Trying to connect...")
				}
			}
		}
	},
}

func handleWatchEvents(events chan local.Event) {
	for e := range events {
		switch e.Type {
		case local.EventFileComplete:
			_, _ = fmt.Fprintf(os.Stdout,
				"+ %s %s %s\n",
				e.Cid,
				e.Path,
				formatBytes(e.Size, false),
			)
		case local.EventFileRemoved:
			_, _ = fmt.Fprintf(os.Stdout, "- %s\n", e.Path)
		}
	}
}
