package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"

	httpapi "github.com/ipfs/go-ipfs-http-client"
	logging "github.com/ipfs/go-log"
	homedir "github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
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

	apiAddrDef              = "/ip4/127.0.0.1/tcp/3006"
	threadsHostAddrDef      = "/ip4/0.0.0.0/tcp/4006"
	threadsHostProxyAddrDef = "/ip4/0.0.0.0/tcp/5006"
	threadsApiAddrDef       = "/ip4/127.0.0.1/tcp/6006"
	threadsApiProxyAddrDef  = "/ip4/127.0.0.1/tcp/7006"
	ipfsApiAddrDef          = "/ip4/127.0.0.1/tcp/5001"

	cfgFile string
)

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(
		&cfgFile,
		"config",
		"",
		"Config file (default $HOME/.textiled/config.yaml)")

	rootCmd.PersistentFlags().BoolP(
		"debug",
		"d",
		false,
		"Enable debug logging")

	rootCmd.PersistentFlags().StringP(
		"repoPath",
		"r",
		"",
		"Path to repository (default $HOME/.textiled/repo)")

	rootCmd.PersistentFlags().String(
		"apiAddr",
		apiAddrDef,
		"Textile API listen address")

	rootCmd.PersistentFlags().String(
		"threadsHostAddr",
		threadsHostAddrDef,
		"Threads peer host listen address")
	rootCmd.PersistentFlags().String(
		"threadsHostProxyAddr",
		threadsHostProxyAddrDef,
		"Threads peer host gRPC proxy address")
	rootCmd.PersistentFlags().String(
		"threadsApiAddr",
		threadsApiAddrDef,
		"Threads API listen address")
	rootCmd.PersistentFlags().String(
		"threadsApiProxyAddr",
		threadsApiProxyAddrDef,
		"Threads API gRPC proxy address")

	rootCmd.PersistentFlags().String(
		"ipfsApiAddr",
		ipfsApiAddrDef,
		"IPFS API address")

	_ = viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	_ = viper.BindPFlag("repo", rootCmd.PersistentFlags().Lookup("repoPath"))
	_ = viper.BindPFlag("api.addr", rootCmd.PersistentFlags().Lookup("apiAddr"))
	_ = viper.BindPFlag("threads.host.addr", rootCmd.PersistentFlags().Lookup("threadsHostAddr"))
	_ = viper.BindPFlag("threads.host.proxy-addr", rootCmd.PersistentFlags().Lookup("threadsHostProxyAddr"))
	_ = viper.BindPFlag("threads.api.addr", rootCmd.PersistentFlags().Lookup("threadsApiAddr"))
	_ = viper.BindPFlag("threads.api.proxy-addr", rootCmd.PersistentFlags().Lookup("threadsApiProxyAddr"))
	_ = viper.BindPFlag("ipfs.addr", rootCmd.PersistentFlags().Lookup("ipfsApiAddr"))

	viper.SetDefault("debug", false)
	viper.SetDefault("repo", "${HOME}/.textiled/repo")
	viper.SetDefault("api.addr", apiAddrDef)
	viper.SetDefault("threads.host.addr", threadsHostAddrDef)
	viper.SetDefault("threads.host.proxy-addr", threadsHostProxyAddrDef)
	viper.SetDefault("threads.api.addr", threadsApiAddrDef)
	viper.SetDefault("threads.api.proxy-addr", threadsApiProxyAddrDef)
	viper.SetDefault("ipfs.addr", ipfsApiAddrDef)
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
		repoFlag := cmd.Flag("repoPath")

		var repoPath string
		if !repoFlag.Changed {
			home, err := homedir.Dir()
			if err != nil {
				log.Fatal(err)
			}
			repoPath = path.Join(home, ".textiled")
		} else {
			repoPath = repoFlag.Value.String()
		}

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
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		viper.AddConfigPath(path.Join(home, ".textiled"))
		viper.SetConfigName("config")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		log.Info("Using config file:", viper.ConfigFileUsed())
	}
}
