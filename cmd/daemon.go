package cmd

import (
	"context"
	"fmt"
	"path"

	"github.com/mitchellh/go-homedir"

	httpapi "github.com/ipfs/go-ipfs-http-client"
	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	"github.com/textileio/go-textile-threads/api"
	es "github.com/textileio/go-textile-threads/eventstore"
	"github.com/textileio/go-textile-threads/util"
)

var log = logging.Logger("textile")

// daemonCmd represents the daemon command.
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the Textile daemon",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		repoFlag := cmd.Flag("repoPath")

		var repoPath string
		if !repoFlag.Changed {
			home, err := homedir.Dir()
			if err != nil {
				log.Fatal(err)
			}
			repoPath = path.Join(home, ".textile")
		} else {
			repoPath = repoFlag.Value.String()
		}

		ipfsApiAddrStr := getStringFlag(cmd.Flag("ipfsApiAddr"))
		ipfsApiAddr, err := ma.NewMultiaddr(ipfsApiAddrStr)
		if err != nil {
			log.Fatal(err)
		}

		threadsHostBindAddrStr := getStringFlag(cmd.Flag("threadsHostBindAddr"))
		threadsHostBindAddr, err := ma.NewMultiaddr(threadsHostBindAddrStr)
		if err != nil {
			log.Fatal(err)
		}
		threadsHostProxyBindAddrStr := getStringFlag(cmd.Flag("threadsHostProxyBindAddr"))
		threadsHostProxyBindAddr, err := ma.NewMultiaddr(threadsHostProxyBindAddrStr)
		if err != nil {
			log.Fatal(err)
		}
		threadsApiBindAddrStr := getStringFlag(cmd.Flag("threadsApiBindAddr"))
		threadsApiBindAddr, err := ma.NewMultiaddr(threadsApiBindAddrStr)
		if err != nil {
			log.Fatal(err)
		}
		threadsApiProxyBindAddrStr := getStringFlag(cmd.Flag("threadsApiProxyBindAddr"))
		threadsApiProxyBindAddr, err := ma.NewMultiaddr(threadsApiProxyBindAddrStr)
		if err != nil {
			log.Fatal(err)
		}

		util.SetupDefaultLoggingConfig(repoPath)
		debug := getBoolFlag(cmd.Flag("debug"))
		if debug {
			if err := logging.SetLogLevel("textile", "debug"); err != nil {
				log.Fatal(err)
			}
		}

		_, err = httpapi.NewApi(ipfsApiAddr)
		if err != nil {
			log.Fatal(err)
		}

		ts, err := es.DefaultThreadservice(
			repoPath,
			es.HostAddr(threadsHostBindAddr),
			es.HostProxyAddr(threadsHostProxyBindAddr),
			es.Debug(debug))
		if err != nil {
			log.Fatal(err)
		}
		defer ts.Close()
		ts.Bootstrap(util.DefaultBoostrapPeers())

		server, err := api.NewServer(context.Background(), ts, api.Config{
			RepoPath:  repoPath,
			Addr:      threadsApiBindAddr,
			ProxyAddr: threadsApiProxyBindAddr,
			Debug:     debug,
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

func init() {
	rootCmd.AddCommand(daemonCmd)
	daemonCmd.Flags().BoolP("debug", "d", false, "Enable debug logging")
	daemonCmd.Flags().StringP("repoPath", "r", "~/.textile", "Path to repository")
	daemonCmd.Flags().String("ipfsApiAddr", "/ip4/127.0.0.1/tcp/5001", "IPFS API address")
	daemonCmd.Flags().String("threadsHostBindAddr", "/ip4/0.0.0.0/tcp/4006", "Threads peer host listen address")
	daemonCmd.Flags().String("threadsHostProxyBindAddr", "/ip4/0.0.0.0/tcp/5006", "Threads peer host gRPC proxy address")
	daemonCmd.Flags().String("threadsApiBindAddr", "/ip4/127.0.0.1/tcp/6006", "Threads API listen address")
	daemonCmd.Flags().String("threadsApiProxyBindAddr", "/ip4/127.0.0.1/tcp/7006", "Threads API gRPC proxy address")
}
