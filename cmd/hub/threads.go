package main

import (
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/cmd"
	buck "github.com/textileio/textile/cmd/buck/cli"
)

func init() {
	rootCmd.AddCommand(threadsCmd)
	threadsCmd.AddCommand(threadsLsCmd)

	threadsCmd.PersistentFlags().String("org", "", "Org username")
}

var threadsCmd = &cobra.Command{
	Use: "threads",
	Aliases: []string{
		"thread",
	},
	Short: "Thread management",
	Long:  `Manages your threads.`,
	Args:  cobra.ExactArgs(0),
}

var threadsLsCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List your threads",
	Long:  `Lists all of your threads.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		org, err := c.Flags().GetString("org")
		if err != nil {
			cmd.Fatal(err)
		}
		if org != "" {
			buck.Config().Viper.Set("org", org)
		}

		ctx, cancel := clients.Ctx.Auth(cmd.Timeout)
		defer cancel()
		list, err := clients.Users.ListThreads(ctx)
		if err != nil {
			cmd.Fatal(err)
		}
		if len(list.List) > 0 {
			data := make([][]string, len(list.List))
			for i, t := range list.List {
				id, err := thread.Cast(t.ID)
				if err != nil {
					cmd.Fatal(err)
				}
				if t.Name == "" {
					t.Name = "unnamed"
				}
				data[i] = []string{id.String(), t.Name, cmd.GetThreadType(t.IsDB)}
			}
			cmd.RenderTable([]string{"id", "name", "type"}, data)
		}
		cmd.Message("Found %d threads", aurora.White(len(list.List)).Bold())
	},
}
