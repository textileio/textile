package main

import (
	"fmt"

	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-textile-threads/util"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/textile/core"
	logger "github.com/whyrusleeping/go-logging"
)

var (
	log = logging.Logger("textiled")

	configFile string
	flags      = map[string]cmd.Flag{
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
		"addrIpfsApi": {
			Key:      "addr.ipfs.api",
			DefValue: "/ip4/127.0.0.1/tcp/5001",
		},
	}
)

func init() {
	cobra.OnInitialize(cmd.InitConfig(configFile, ".textiled", func() {
		log.Debugf("Using config file: %s", viper.ConfigFileUsed())
	}))

	rootCmd.PersistentFlags().StringVar(
		&configFile,
		"config",
		"",
		"Config file (default ${HOME}/.textiled/config.yaml)")

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

	if err := cmd.BindFlags(rootCmd, flags); err != nil {
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
		cmd.ExpandConfigVars(flags)

		if viper.GetBool("log.debug") {
			if err := util.SetLogLevels(map[string]logger.Level{
				"textiled": logger.DEBUG,
			}); err != nil {
				log.Fatal(err)
			}
		}
	},
	Run: func(c *cobra.Command, args []string) {
		addrApi, err := ma.NewMultiaddr(viper.GetString("addr.api"))
		if err != nil {
			log.Fatal(err)
		}

		addrThreadsHost, err := ma.NewMultiaddr(viper.GetString("addr.threads.host"))
		if err != nil {
			log.Fatal(err)
		}
		addrThreadsHostProxy, err := ma.NewMultiaddr(viper.GetString("addr.threads.host_proxy"))
		if err != nil {
			log.Fatal(err)
		}
		addrThreadsApi, err := ma.NewMultiaddr(viper.GetString("addr.threads.api"))
		if err != nil {
			log.Fatal(err)
		}
		addrThreadsApiProxy, err := ma.NewMultiaddr(viper.GetString("addr.threads.api_proxy"))
		if err != nil {
			log.Fatal(err)
		}

		addrIpfsApi, err := ma.NewMultiaddr(viper.GetString("addr.ipfs.api"))
		if err != nil {
			log.Fatal(err)
		}

		logFile := viper.GetString("log.file")
		if logFile != "" {
			util.SetupDefaultLoggingConfig(logFile)
		}

		textile, err := core.NewTextile(core.Config{
			RepoPath:             viper.GetString("repo"),
			AddrApi:              addrApi,
			AddrThreadsHost:      addrThreadsHost,
			AddrThreadsHostProxy: addrThreadsHostProxy,
			AddrThreadsApi:       addrThreadsApi,
			AddrThreadsApiProxy:  addrThreadsApiProxy,
			AddrIpfsApi:          addrIpfsApi,
			Debug:                viper.GetBool("log.debug"),
		})
		if err != nil {
			log.Fatal(err)
		}
		defer textile.Close()
		textile.Bootstrap()

		fmt.Println("Welcome to Textile!")
		fmt.Println("Your peer ID is " + textile.HostID().String())

		log.Debug("textiled started")

		select {}
	},
}
