package cli

import (
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
	buck "github.com/textileio/textile/cmd/buck/cli"
)

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

		threads := clients.ListThreads(false)
		if len(threads) > 0 {
			data := make([][]string, len(threads))
			for i, t := range threads {
				data[i] = []string{t.ID.String(), t.Name, t.Type}
			}
			cmd.RenderTable([]string{"id", "name", "type"}, data)
		}
		cmd.Message("Found %d threads", aurora.White(len(threads)).Bold())
	},
}
