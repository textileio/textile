package main

import (
	"context"
	"encoding/json"
	"fmt"

	logging "github.com/ipfs/go-log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/textile/core"
)

var (
	log = logging.Logger("textiled")

	configViper = viper.New()
	configFile  string

	flags = map[string]cmd.Flag{
		"repo": {
			Key:      "repo",
			DefValue: "${HOME}/.textiled/repo",
		},
		"debug": {
			Key:      "log.debug",
			DefValue: false,
		},
		"logFile": {
			Key:      "log.file",
			DefValue: "${HOME}/.textiled/log",
		},
		"addrApi": {
			Key:      "addr.api",
			DefValue: "/ip4/127.0.0.1/tcp/3006",
		},
		"addrThreadsHost": {
			Key:      "addr.threads.host",
			DefValue: "/ip4/0.0.0.0/tcp/4006",
		},
		"addrThreadsHostProxy": {
			Key:      "addr.threads.host_proxy",
			DefValue: "/ip4/0.0.0.0/tcp/5006",
		},
		"addrThreadsApi": {
			Key:      "addr.threads.api",
			DefValue: "/ip4/127.0.0.1/tcp/6006",
		},
		"addrThreadsApiProxy": {
			Key:      "addr.threads.api_proxy",
			DefValue: "/ip4/127.0.0.1/tcp/7006",
		},
		"addrGatewayHost": {
			Key:      "addr.gateway.host",
			DefValue: "/ip4/127.0.0.1/tcp/8006",
		},
		"addrGatewayUrl": {
			Key:      "addr.gateway.url",
			DefValue: "http://127.0.0.1:8006",
		},
		"addrIpfsApi": {
			Key:      "addr.ipfs.api",
			DefValue: "/ip4/127.0.0.1/tcp/5001",
		},
		"addrFilecoinApi": {
			Key:      "addr.filecoin.api",
			DefValue: "/ip4/127.0.0.1/tcp/5002",
		},
		"emailFrom": {
			Key:      "email.from",
			DefValue: "Textile <verify@email.textile.io>",
		},
		"emailDomain": {
			Key:      "email.domain",
			DefValue: "email.textile.io",
		},
		"emailApiKey": {
			Key:      "email.api_key",
			DefValue: "",
		},
	}
)

func init() {
	cobra.OnInitialize(cmd.InitConfig(configViper, configFile, ".textiled", "config"))

	rootCmd.PersistentFlags().StringVar(
		&configFile,
		"config",
		"",
		"Config file (default ${HOME}/.textiled/config.yml)")

	rootCmd.PersistentFlags().StringP(
		"repo",
		"r",
		flags["repo"].DefValue.(string),
		"Path to repository")

	rootCmd.PersistentFlags().BoolP(
		"debug",
		"d",
		flags["debug"].DefValue.(bool),
		"Enable debug logging")

	rootCmd.PersistentFlags().String(
		"logFile",
		flags["logFile"].DefValue.(string),
		"Write logs to file")

	rootCmd.PersistentFlags().String(
		"addrApi",
		flags["addrApi"].DefValue.(string),
		"Textile API listen address")

	rootCmd.PersistentFlags().String(
		"addrThreadsHost",
		flags["addrThreadsHost"].DefValue.(string),
		"Threads peer host listen address")
	rootCmd.PersistentFlags().String(
		"addrThreadsHostProxy",
		flags["addrThreadsHostProxy"].DefValue.(string),
		"Threads peer host gRPC proxy address")
	rootCmd.PersistentFlags().String(
		"addrThreadsApi",
		flags["addrThreadsApi"].DefValue.(string),
		"Threads API listen address")
	rootCmd.PersistentFlags().String(
		"addrThreadsApiProxy",
		flags["addrThreadsApiProxy"].DefValue.(string),
		"Threads API gRPC proxy address")

	rootCmd.PersistentFlags().String(
		"addrIpfsApi",
		flags["addrIpfsApi"].DefValue.(string),
		"IPFS API address")

	// Gateway settings
	rootCmd.PersistentFlags().String(
		"addrGatewayHost",
		flags["addrGatewayHost"].DefValue.(string),
		"Local gateway host address")
	rootCmd.PersistentFlags().String(
		"addrGatewayUrl",
		flags["addrGatewayUrl"].DefValue.(string),
		"Public gateway address")

	// Filecoin settings
	rootCmd.PersistentFlags().String(
		"addrFilecoinApi",
		flags["addrFilecoinApi"].DefValue.(string),
		"Filecoin gRPC API address")

	// Verification email settings
	rootCmd.PersistentFlags().String(
		"emailFrom",
		flags["emailFrom"].DefValue.(string),
		"Source address of system emails")

	rootCmd.PersistentFlags().String(
		"emailDomain",
		flags["emailDomain"].DefValue.(string),
		"Domain of system emails")

	rootCmd.PersistentFlags().String(
		"emailApiKey",
		flags["emailApiKey"].DefValue.(string),
		"Mailgun API key for sending emails")

	if err := cmd.BindFlags(configViper, rootCmd, flags); err != nil {
		log.Fatal(err)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		cmd.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   "textiled",
	Short: "Textile daemon",
	Long:  `The Textile daemon.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		configViper.SetConfigType("yaml")
		cmd.ExpandConfigVars(configViper, flags)

		if configViper.GetBool("log.debug") {
			if err := util.SetLogLevels(map[string]logging.LogLevel{
				"textiled": logging.LevelDebug,
			}); err != nil {
				log.Fatal(err)
			}
		}
	},
	Run: func(c *cobra.Command, args []string) {
		settings, err := json.MarshalIndent(configViper.AllSettings(), "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		log.Debugf("loaded config: %s", string(settings))

		addrApi := cmd.AddrFromStr(configViper.GetString("addr.api"))
		addrThreadsHost := cmd.AddrFromStr(configViper.GetString("addr.threads.host"))
		addrThreadsHostProxy := cmd.AddrFromStr(configViper.GetString("addr.threads.host_proxy"))
		addrThreadsApi := cmd.AddrFromStr(configViper.GetString("addr.threads.api"))
		addrThreadsApiProxy := cmd.AddrFromStr(configViper.GetString("addr.threads.api_proxy"))
		addrIpfsApi := cmd.AddrFromStr(configViper.GetString("addr.ipfs.api"))

		addrGatewayHost := cmd.AddrFromStr(configViper.GetString("addr.gateway.host"))
		addrGatewayUrl := configViper.GetString("addr.gateway.url")

		addrFilecoinApi := cmd.AddrFromStr(configViper.GetString("addr.filecoin.api"))

		emailFrom := configViper.GetString("email.from")
		emailDomain := configViper.GetString("email.domain")
		emailApiKey := configViper.GetString("email.api_key")

		logFile := configViper.GetString("log.file")
		if logFile != "" {
			util.SetupDefaultLoggingConfig(logFile)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		textile, err := core.NewTextile(ctx, core.Config{
			RepoPath:             configViper.GetString("repo"),
			AddrApi:              addrApi,
			AddrThreadsHost:      addrThreadsHost,
			AddrThreadsHostProxy: addrThreadsHostProxy,
			AddrThreadsApi:       addrThreadsApi,
			AddrThreadsApiProxy:  addrThreadsApiProxy,
			AddrIpfsApi:          addrIpfsApi,
			AddrGatewayHost:      addrGatewayHost,
			AddrGatewayUrl:       addrGatewayUrl,
			AddrFilecoinApi:      addrFilecoinApi,
			EmailFrom:            emailFrom,
			EmailDomain:          emailDomain,
			EmailApiKey:          emailApiKey,
			Debug:                configViper.GetBool("log.debug"),
		})
		if err != nil {
			log.Fatal(err)
		}
		defer textile.Close()
		textile.Bootstrap()

		fmt.Println("Welcome to Textile!")
		fmt.Println("Your peer ID is " + textile.HostID().String())

		select {}
	},
}
