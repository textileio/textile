package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	logging "github.com/ipfs/go-log"
	homedir "github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-textile-threads/util"
	"github.com/textileio/textile/core"
)

var (
	log = logging.Logger("textiled")

	configFile string
	flags      = map[string]flag{
		"repo": {
			key:      "repo",
			defValue: "${HOME}/.textiled/repo",
		},

		"debug": {
			key:      "log.debug",
			defValue: false,
		},

		"logFile": {
			key:      "log.file",
			defValue: "${HOME}/.textiled/log",
		},

		"addrApi": {
			key:      "addr.api",
			defValue: "/ip4/127.0.0.1/tcp/3006",
		},
		"addrThreadsHost": {
			key:      "addr.threads.host",
			defValue: "/ip4/0.0.0.0/tcp/4006",
		},
		"addrThreadsHostProxy": {
			key:      "addr.threads.host_proxy",
			defValue: "/ip4/0.0.0.0/tcp/5006",
		},
		"addrThreadsApi": {
			key:      "addr.threads.api",
			defValue: "/ip4/127.0.0.1/tcp/6006",
		},
		"addrThreadsApiProxy": {
			key:      "addr.threads.api_proxy",
			defValue: "/ip4/127.0.0.1/tcp/7006",
		},
		"addrIpfsApi": {
			key:      "addr.ipfs.api",
			defValue: "/ip4/127.0.0.1/tcp/5001",
		},

		"usersStoreId": {
			key: "users.store.id",
		},
	}
)

type flag struct {
	key      string
	defValue interface{}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(
		&configFile,
		"config",
		"",
		"Config file (default ${HOME}/.textiled/config.yaml)")

	rootCmd.PersistentFlags().StringP(
		"repo",
		"r",
		flags["repo"].defValue.(string),
		"Path to repository")

	rootCmd.PersistentFlags().BoolP(
		"debug",
		"d",
		flags["debug"].defValue.(bool),
		"Enable debug logging")

	rootCmd.PersistentFlags().String(
		"logFile",
		flags["logFile"].defValue.(string),
		"Write logs to file")

	rootCmd.PersistentFlags().String(
		"addrApi",
		flags["addrApi"].defValue.(string),
		"Textile API listen address")

	rootCmd.PersistentFlags().String(
		"addrThreadsHost",
		flags["addrThreadsHost"].defValue.(string),
		"Threads peer host listen address")
	rootCmd.PersistentFlags().String(
		"addrThreadsHostProxy",
		flags["addrThreadsHostProxy"].defValue.(string),
		"Threads peer host gRPC proxy address")
	rootCmd.PersistentFlags().String(
		"addrThreadsApi",
		flags["addrThreadsApi"].defValue.(string),
		"Threads API listen address")
	rootCmd.PersistentFlags().String(
		"addrThreadsApiProxy",
		flags["addrThreadsApiProxy"].defValue.(string),
		"Threads API gRPC proxy address")

	rootCmd.PersistentFlags().String(
		"addrIpfsApi",
		flags["addrIpfsApi"].defValue.(string),
		"IPFS API address")

	rootCmd.PersistentFlags().String(
		"usersStoreId",
		"",
		"Users store ID")

	for n, f := range flags {
		if err := viper.BindPFlag(f.key, rootCmd.PersistentFlags().Lookup(n)); err != nil {
			log.Fatal(err)
		}
		viper.SetDefault(f.key, f.defValue)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "textiled",
	Short: "Textile daemon",
	Long:  `The Textile daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Expand environment variables in config
		for _, f := range flags {
			if f.key != "" {
				if str, ok := viper.Get(f.key).(string); ok {
					viper.Set(f.key, os.ExpandEnv(str))
				}
			}
		}

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

func initConfig() {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		viper.AddConfigPath(path.Join(home, ".textiled"))
		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("TXTL")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		log.Info("Using config file:", viper.ConfigFileUsed())
	}
}
