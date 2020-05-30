package main

import (
	"context"
	"errors"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/cmd"
	buck "github.com/textileio/textile/cmd/buck/cli"
	hub "github.com/textileio/textile/cmd/hub/cli"
)

var clients *cmd.Clients

func init() {
	cobra.OnInitialize(cmd.InitConfig(hub.Config()))
	cobra.OnInitialize(cmd.InitConfig(buck.Config()))
	hub.Init(rootCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		cmd.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   hub.Name,
	Short: "Hub Client",
	Long:  `The Hub Client.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		hub.Config().Viper.SetConfigType("yaml")
		buck.Config().Viper.SetConfigType("yaml")

		cmd.ExpandConfigVars(hub.Config().Viper, hub.Config().Flags)

		if hub.Config().Viper.GetString("session") == "" && c.Use != "init" && c.Use != "login" {
			msg := "unauthorized! run `%s` or use `%s` to authorize"
			cmd.Fatal(errors.New(msg), aurora.Cyan(hub.Name+" init|login"), aurora.Cyan("--session"))
		}

		clients = cmd.NewClients(hub.Config().Viper.GetString("api"), true, &ctx{})
		hub.SetClients(clients)
		buck.SetClients(clients)
	},
	PersistentPostRun: func(c *cobra.Command, args []string) {
		clients.Close()
	},
	Args: cobra.ExactArgs(0),
}

type ctx struct{}

func (c *ctx) Auth(duration time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	ctx = common.NewSessionContext(ctx, hub.Config().Viper.GetString("session"))
	ctx = common.NewOrgSlugContext(ctx, buck.Config().Viper.GetString("org"))
	return ctx, cancel
}

func (c *ctx) Thread(duration time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := c.Auth(duration)
	ctx = common.NewThreadIDContext(ctx, cmd.ThreadIDFromString(buck.Config().Viper.GetString("thread")))
	return ctx, cancel
}
