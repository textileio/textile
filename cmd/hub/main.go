package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
	buck "github.com/textileio/textile/v2/cmd/buck/cli"
	hub "github.com/textileio/textile/v2/cmd/hub/cli"
)

var clients *cmd.Clients

func init() {
	cobra.OnInitialize(cmd.InitConfig(hub.Config()))
	hub.Init(rootCmd)
}

func main() {
	cmd.ErrCheck(rootCmd.Execute())
}

var rootCmd = &cobra.Command{
	Use:   hub.Name,
	Short: "Hub Client",
	Long:  `The Hub Client.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(hub.Config().Viper, hub.Config().Flags)

		if hub.Config().Viper.GetString("session") == "" && c.Use != "init" && c.Use != "login" && c.Use != "version" && c.Use != "update" {
			msg := "unauthorized! run `%s` or use `%s` to authorize"
			cmd.Fatal(errors.New(msg), aurora.Cyan(hub.Name+" init|login"), aurora.Cyan("--session"))
		}

		clients = cmd.NewClients(hub.Config().Viper.GetString("api"), true)
		hub.SetClients(clients)
		config := local.DefaultConfConfig()
		config.EnvPrefix = fmt.Sprintf("%s_%s", strings.ToUpper(hub.Name), strings.ToUpper(buck.Name))
		buck.SetBucks(local.NewBucketsWithAuth(clients, config, hub.Auth))
	},
	PersistentPostRun: func(c *cobra.Command, args []string) {
		clients.Close()
	},
	Args: cobra.ExactArgs(0),
}
