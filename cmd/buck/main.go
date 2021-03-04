package main

import (
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
	buck "github.com/textileio/textile/v2/cmd/buck/cli"
)

const defaultTarget = "127.0.0.1:3006"

var clients *cmd.Clients

func init() {
	buck.Init(rootCmd)

	rootCmd.PersistentFlags().String("api", defaultTarget, "API target")
}

func main() {
	cmd.ErrCheck(rootCmd.Execute())
}

var rootCmd = &cobra.Command{
	Use:   buck.Name,
	Short: "Bucket Client",
	Long: `The Bucket Client.

Manages files and folders in an object storage bucket.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		config := local.DefaultConfConfig()
		hubTarget := cmd.GetFlagOrEnvValue(c, "api", config.EnvPrefix)
		minerIndexTarget := cmd.GetFlagOrEnvValue(c, "apimindex", config.EnvPrefix)

		clients = cmd.NewClients(hubTarget, false, minerIndexTarget)
		buck.SetBucks(local.NewBuckets(clients, config))
	},
	PersistentPostRun: func(c *cobra.Command, args []string) {
		clients.Close()
	},
	Args: cobra.ExactArgs(0),
}
