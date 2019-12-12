package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	httpapi "github.com/ipfs/go-ipfs-http-client"
	logging "github.com/ipfs/go-log"
	homedir "github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	threadsapi "github.com/textileio/go-textile-threads/api"
	threadsclient "github.com/textileio/go-textile-threads/api/client"
	es "github.com/textileio/go-textile-threads/eventstore"
	"github.com/textileio/go-textile-threads/util"
	"github.com/textileio/textile/api"
	common "github.com/textileio/textile/cmd"
)

var (
	log = logging.Logger("textiled")

	configFile string
	configKeys = map[string]configKey{
		"debug": {
			key:      "debug",
			defValue: false,
		},
		"repoPath": {
			key: "repo",
		},
		"apiAddr": {
			key:      "api.addr",
			defValue: "/ip4/127.0.0.1/tcp/3006",
		},
		"threadsHostAddr": {
			key:      "threads.host.addr",
			defValue: "/ip4/0.0.0.0/tcp/4006",
		},
		"threadsHostProxyAddr": {
			key:      "threads.host.proxy-addr",
			defValue: "/ip4/0.0.0.0/tcp/5006",
		},
		"threadsApiAddr": {
			key:      "threads.api.addr",
			defValue: "/ip4/127.0.0.1/tcp/6006",
		},
		"threadsApiProxyAddr": {
			key:      "threads.api.proxy-addr",
			defValue: "/ip4/127.0.0.1/tcp/7006",
		},
		"ipfsApiAddr": {
			key:      "ipfs.addr",
			defValue: "/ip4/127.0.0.1/tcp/5001",
		},
	}
)

type configKey struct {
	key      string
	defValue interface{}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(
		&configFile,
		"config",
		"",
		"Config file (default $HOME/.textiled/config.yaml)")

	rootCmd.PersistentFlags().StringP(
		"repoPath",
		"r",
		"",
		"Path to repository (default $HOME/.textiled/repo)")

	rootCmd.PersistentFlags().BoolP(
		"debug",
		"d",
		configKeys["debug"].defValue.(bool),
		"Enable debug logging")

	rootCmd.PersistentFlags().String(
		"apiAddr",
		configKeys["apiAddr"].defValue.(string),
		"Textile API listen address")

	rootCmd.PersistentFlags().String(
		"threadsHostAddr",
		configKeys["threadsHostAddr"].defValue.(string),
		"Threads peer host listen address")
	rootCmd.PersistentFlags().String(
		"threadsHostProxyAddr",
		configKeys["threadsHostProxyAddr"].defValue.(string),
		"Threads peer host gRPC proxy address")
	rootCmd.PersistentFlags().String(
		"threadsApiAddr",
		configKeys["threadsApiAddr"].defValue.(string),
		"Threads API listen address")
	rootCmd.PersistentFlags().String(
		"threadsApiProxyAddr",
		configKeys["threadsApiProxyAddr"].defValue.(string),
		"Threads API gRPC proxy address")

	rootCmd.PersistentFlags().String(
		"ipfsApiAddr",
		configKeys["ipfsApiAddr"].defValue.(string),
		"IPFS API address")

	for n, k := range configKeys {
		if err := viper.BindPFlag(k.key, rootCmd.PersistentFlags().Lookup(n)); err != nil {
			log.Fatal(err)
		}
		viper.SetDefault(k.key, k.defValue)
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
		cmd.Flags().VisitAll(func(flag *pflag.Flag) {
			if !flag.Changed {
				configVal := viper.GetString(configKeys[flag.Name].key)
				if configVal != "" {
					if err := cmd.Flag(flag.Name).Value.Set(os.ExpandEnv(configVal)); err != nil {
						log.Fatal(err)
					}
				}
			}
		})

		apiAddr, err := ma.NewMultiaddr(cmd.Flag("apiAddr").Value.String())
		if err != nil {
			log.Fatal(err)
		}

		threadsHostAddr, err := ma.NewMultiaddr(cmd.Flag("threadsHostAddr").Value.String())
		if err != nil {
			log.Fatal(err)
		}
		threadsHostProxyAddr, err := ma.NewMultiaddr(cmd.Flag("threadsHostProxyAddr").Value.String())
		if err != nil {
			log.Fatal(err)
		}
		threadsApiAddr, err := ma.NewMultiaddr(cmd.Flag("threadsApiAddr").Value.String())
		if err != nil {
			log.Fatal(err)
		}
		threadsApiProxyAddr, err := ma.NewMultiaddr(cmd.Flag("threadsApiProxyAddr").Value.String())
		if err != nil {
			log.Fatal(err)
		}

		ipfsApiAddr, err := ma.NewMultiaddr(cmd.Flag("ipfsApiAddr").Value.String())
		if err != nil {
			log.Fatal(err)
		}

		repoPath := cmd.Flag("repoPath").Value.String()
		util.SetupDefaultLoggingConfig(repoPath)
		debug := common.GetBoolFlag(cmd.Flag("debug"))
		if debug {
			if err := logging.SetLogLevel("textiled", "debug"); err != nil {
				log.Fatal(err)
			}
		}

		_, err = httpapi.NewApi(ipfsApiAddr)
		if err != nil {
			log.Fatal(err)
		}

		ts, err := es.DefaultThreadservice(
			repoPath,
			es.HostAddr(threadsHostAddr),
			es.HostProxyAddr(threadsHostProxyAddr),
			es.Debug(debug))
		if err != nil {
			log.Fatal(err)
		}
		defer ts.Close()
		ts.Bootstrap(util.DefaultBoostrapPeers())

		threadsServer, err := threadsapi.NewServer(context.Background(), ts, threadsapi.Config{
			RepoPath:  repoPath,
			Addr:      threadsApiAddr,
			ProxyAddr: threadsApiProxyAddr,
			Debug:     debug,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer threadsServer.Close()

		// @todo: Threads Client should take a multiaddress.
		threadsHost, err := threadsApiAddr.ValueForProtocol(ma.P_IP4)
		if err != nil {
			log.Fatal(err)
		}
		threadsPortStr, err := threadsApiAddr.ValueForProtocol(ma.P_TCP)
		if err != nil {
			log.Fatal(err)
		}
		threadsPort, err := strconv.Atoi(threadsPortStr)
		if err != nil {
			log.Fatal(err)
		}

		threadsClient, err := threadsclient.NewClient(threadsHost, threadsPort)
		if err != nil {
			log.Fatal(err)
		}

		server, err := api.NewServer(context.Background(), threadsClient, api.Config{
			Addr:  apiAddr,
			Debug: debug,
		})
		if err != nil {
			log.Fatal(err)
		}
		defer server.Close()

		fmt.Println("Welcome to Textile!")
		fmt.Println("Your peer ID is " + ts.Host().ID().String())

		log.Debug("daemon started")

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
