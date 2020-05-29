package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/cmd"
	buck "github.com/textileio/textile/cmd/buck/cli"
)

const defaultTarget = "127.0.0.1:3006"

var clients *cmd.Clients

func init() {
	cobra.OnInitialize(cmd.InitConfig(buck.Config()))
	buck.Init(rootCmd)

	rootCmd.PersistentFlags().String("api", defaultTarget, "API target")
	if err := buck.Config().Viper.BindPFlag("api", rootCmd.PersistentFlags().Lookup("api")); err != nil {
		cmd.Fatal(err)
	}
	buck.Config().Viper.SetDefault("api", defaultTarget)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		cmd.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   buck.Name,
	Short: "Bucket Client",
	Long: `The Bucket Client.

Manages files and folders in an object storage bucket.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		buck.Config().Viper.SetConfigType("yaml")

		target, err := c.Flags().GetString("api")
		if err != nil {
			cmd.Fatal(err)
		}
		clients = cmd.NewClients(target, false, &ctx{})
		buck.SetClients(clients)
	},
	PersistentPostRun: func(c *cobra.Command, args []string) {
		clients.Close()
	},
	Args: cobra.ExactArgs(0),
}

type ctx struct{}

func (c *ctx) Auth(duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), duration)
}

func (c *ctx) Thread(duration time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := c.Auth(duration)
	ctx = common.NewThreadIDContext(ctx, cmd.ThreadIDFromString(buck.Config().Viper.GetString("thread")))
	return ctx, cancel
}
