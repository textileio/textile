package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/textile/cmd/buck/cli"
)

const (
	cliName       = "buck"
	defaultTarget = "127.0.0.1:3006"
)

func init() {
	cobra.OnInitialize(cmd.InitConfig(cli.Config()))
	cli.Init(rootCmd)

	rootCmd.PersistentFlags().String("api", defaultTarget, "API target")
	if err := cli.Config().Viper.BindPFlag("api", rootCmd.PersistentFlags().Lookup("api")); err != nil {
		cmd.Fatal(err)
	}
	cli.Config().Viper.SetDefault("api", defaultTarget)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		cmd.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   cliName,
	Short: "Bucket Client",
	Long: `The Bucket Client.

Manages files and folders in an object storage bucket.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		cli.Config().Viper.SetConfigType("yaml")

		target, err := c.Flags().GetString("api")
		if err != nil {
			cmd.Fatal(err)
		}
		cli.SetClients(cmd.NewClients(target, false, &ctx{}))
	},
	PersistentPostRun: func(c *cobra.Command, args []string) {
		cli.CloseClients()
	},
	Args: cobra.ExactArgs(0),
}

type ctx struct{}

func (c *ctx) Auth(duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), duration)
}

func (c *ctx) Thread(duration time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := c.Auth(duration)
	ctx = common.NewThreadIDContext(ctx, cmd.ThreadIDFromString(cli.Config().Viper.GetString("thread")))
	return ctx, cancel
}
