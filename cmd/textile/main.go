package main

import (
	"context"
	"errors"
	"time"

	logging "github.com/ipfs/go-log"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/util"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/cmd"
)

var (
	log = logging.Logger("textile")

	authFile  string
	authViper = viper.New()

	authFlags = map[string]cmd.Flag{
		"token": {
			Key:      "token",
			DefValue: "",
		},
	}

	configFile  string
	configViper = viper.New()

	flags = map[string]cmd.Flag{
		"debug": {
			Key:      "log.debug",
			DefValue: false,
		},
		"id": {
			Key:      "id",
			DefValue: "",
		},
		"store": {
			Key:      "store",
			DefValue: "",
		},
		"scope": {
			Key:      "scope",
			DefValue: "",
		},
		"addrApi": {
			Key:      "addr.api",
			DefValue: "/ip4/127.0.0.1/tcp/3006",
		},
	}

	client *api.Client

	cmdTimeout   = time.Second * 10
	loginTimeout = time.Minute * 3
)

func init() {
	rootCmd.AddCommand(whoamiCmd, switchCmd)

	cobra.OnInitialize(cmd.InitConfig(authViper, authFile, ".textile", "auth"))
	cobra.OnInitialize(cmd.InitConfig(configViper, configFile, ".textile", "config"))

	rootCmd.PersistentFlags().StringP(
		"token",
		"t",
		authFlags["token"].DefValue.(string),
		"Authorization token")

	if err := cmd.BindFlags(authViper, rootCmd, authFlags); err != nil {
		cmd.Fatal(err)
	}

	rootCmd.PersistentFlags().StringVar(
		&configFile,
		"config",
		"",
		"Config file (default ${HOME}/.textile/config.yml)")

	rootCmd.PersistentFlags().BoolP(
		"debug",
		"d",
		flags["debug"].DefValue.(bool),
		"Enable debug logging")

	rootCmd.PersistentFlags().String(
		"id",
		flags["id"].DefValue.(string),
		"Project ID")

	rootCmd.PersistentFlags().String(
		"store",
		flags["store"].DefValue.(string),
		"Project Store ID")

	rootCmd.PersistentFlags().String(
		"scope",
		flags["scope"].DefValue.(string),
		"Scope (User or Team ID)")

	rootCmd.PersistentFlags().String(
		"addrApi",
		flags["addrApi"].DefValue.(string),
		"Textile API address")

	if err := cmd.BindFlags(configViper, rootCmd, flags); err != nil {
		cmd.Fatal(err)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		cmd.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   "textile",
	Short: "Textile client",
	Long:  `The Textile client.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		authViper.SetConfigType("yaml")
		configViper.SetConfigType("yaml")

		cmd.ExpandConfigVars(authViper, flags)
		cmd.ExpandConfigVars(configViper, flags)

		if authViper.GetString("token") == "" && c.Use != "login" {
			msg := "unauthorized! run `%s` or use `%s` to authorize"
			cmd.Fatal(errors.New(msg),
				aurora.Cyan("textile login"), aurora.Cyan("--token"))
		}

		if configViper.GetBool("log.debug") {
			if err := util.SetLogLevels(map[string]logging.LogLevel{
				"textile": logging.LevelDebug,
			}); err != nil {
				cmd.Fatal(err)
			}
		}

		var err error
		client, err = api.NewClient(cmd.AddrFromStr(configViper.GetString("addr.api")))
		if err != nil {
			cmd.Fatal(err)
		}
	},
	PersistentPostRun: func(c *cobra.Command, args []string) {
		if client != nil {
			if err := client.Close(); err != nil {
				cmd.Fatal(err)
			}
		}
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show user or team",
	Long:  `Show the user or team for the current session.`,
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		who, err := client.Whoami(
			ctx,
			api.Auth{
				Token: authViper.GetString("token"),
				Scope: configViper.GetString("scope"),
			})
		if err != nil {
			cmd.Fatal(err)
		}

		if who.TeamID != "" {
			cmd.Message("You are %s in the %s team",
				aurora.White(who.Email).Bold(), aurora.White(who.TeamName).Bold())
		} else {
			cmd.Message("You are %s", aurora.White(who.Email).Bold())
		}
	},
}

var switchCmd = &cobra.Command{
	Use:   "switch",
	Short: "Switch teams or personal account",
	Long:  `Switch between teams and your personal account.`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectTeam("Switch to", aurora.Sprintf(
			aurora.BrightBlack("> Switching to {{ .Name | white | bold }}")),
			true)

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.Switch(
			ctx,
			api.Auth{
				Token: authViper.GetString("token"),
				Scope: selected.ID,
			}); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Switched to %s", aurora.White(selected.Name).Bold())
	},
}
