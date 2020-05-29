package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/textile/cmd/buck/cmds"
)

const (
	cliName       = "buck"
	defaultTarget = "127.0.0.1:3006"
)

func init() {
	cobra.OnInitialize(cmd.InitConfig(cmds.Config))
	cmds.InitCmd(rootCmd)

	rootCmd.PersistentFlags().String("api", defaultTarget, "API target")
	if err := cmds.Config.Viper.BindPFlag("api", rootCmd.PersistentFlags().Lookup("api")); err != nil {
		cmd.Fatal(err)
	}
	cmds.Config.Viper.SetDefault("api", defaultTarget)
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
		cmds.Config.Viper.SetConfigType("yaml")

		target, err := c.Flags().GetString("api")
		if err != nil {
			cmd.Fatal(err)
		}
		cmds.Clients = cmd.NewClients(target, false, &ctx{})
	},
	PersistentPostRun: func(c *cobra.Command, args []string) {
		cmds.Clients.Close()
	},
	Args: cobra.ExactArgs(0),
}

type ctx struct{}

func (c *ctx) Auth(duration time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), duration)
}

func (c *ctx) Thread(duration time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := c.Auth(duration)
	ctx = common.NewThreadIDContext(ctx, cmd.ThreadIDFromString(cmds.Config.Viper.GetString("thread")))
	return ctx, cancel
}
