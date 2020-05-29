package main

import (
	"context"
	"errors"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/cmd"
	buck "github.com/textileio/textile/cmd/buck/cli"
)

const (
	cliName = "hub"
)

var (
	config = cmd.Config{
		Viper: viper.New(),
		Dir:   ".textile",
		Name:  "auth",
		Flags: map[string]cmd.Flag{
			"api": {
				Key:      "api",
				DefValue: "api.textile.io:443",
			},
			"session": {
				Key:      "session",
				DefValue: "",
			},
		},
		EnvPre: "HUB",
		Global: true,
	}

	clients *cmd.Clients

	confirmTimeout = time.Hour
)

func init() {
	cobra.OnInitialize(cmd.InitConfig(config))
	cobra.OnInitialize(cmd.InitConfig(buck.Config()))

	rootCmd.PersistentFlags().String(
		"api",
		config.Flags["api"].DefValue.(string),
		"API target")

	rootCmd.PersistentFlags().StringP(
		"session",
		"s",
		config.Flags["session"].DefValue.(string),
		"User session token")

	if err := cmd.BindFlags(config.Viper, rootCmd, config.Flags); err != nil {
		cmd.Fatal(err)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		cmd.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   cliName,
	Short: "Hub Client",
	Long:  `The Hub Client.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		config.Viper.SetConfigType("yaml")
		buck.Config().Viper.SetConfigType("yaml")

		cmd.ExpandConfigVars(config.Viper, config.Flags)

		if config.Viper.GetString("session") == "" && c.Use != "init" && c.Use != "login" {
			msg := "unauthorized! run `%s` or use `%s` to authorize"
			cmd.Fatal(errors.New(msg), aurora.Cyan(cliName+" init|login"), aurora.Cyan("--session"))
		}

		clients = cmd.NewClients(config.Viper.GetString("api"), true, &ctx{})
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
	ctx = common.NewSessionContext(ctx, config.Viper.GetString("session"))
	ctx = common.NewOrgSlugContext(ctx, buck.Config().Viper.GetString("org"))
	return ctx, cancel
}

func (c *ctx) Thread(duration time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := c.Auth(duration)
	ctx = common.NewThreadIDContext(ctx, cmd.ThreadIDFromString(buck.Config().Viper.GetString("thread")))
	return ctx, cancel
}
