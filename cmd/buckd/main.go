package main

import (
	"context"
	"encoding/json"
	"fmt"

	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/textile/core"
)

const (
	daemonName = "bucketsd"
)

var (
	log = logging.Logger(daemonName)

	configFile  string
	configDir   = "." + daemonName
	configViper = viper.New()

	flags = map[string]cmd.Flag{
		"repo": {
			Key:      "repo",
			DefValue: "${HOME}/" + configDir + "/repo",
		},
		"debug": {
			Key:      "log.debug",
			DefValue: false,
		},
		"logFile": {
			Key:      "log.file",
			DefValue: "${HOME}/" + configDir + "/log",
		},
		"addrApi": {
			Key:      "addr.api",
			DefValue: "/ip4/127.0.0.1/tcp/3006",
		},
		"addrApiProxy": {
			Key:      "addr.api_proxy",
			DefValue: "/ip4/127.0.0.1/tcp/3007",
		},
		"addrThreadsHost": {
			Key:      "addr.threads.host",
			DefValue: "/ip4/0.0.0.0/tcp/4006",
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
			DefValue: "",
		},
		"addrMongoUri": {
			Key:      "addr.mongo_uri",
			DefValue: "mongodb://127.0.0.1:27017",
		},
		"gatewaySubdomains": {
			Key:      "gateway.subdomains",
			DefValue: false,
		},
		"dnsDomain": {
			Key:      "dns.domain",
			DefValue: "",
		},
		"dnsZoneID": {
			Key:      "dns.zone_id",
			DefValue: "",
		},
		"dnsToken": {
			Key:      "dns.token",
			DefValue: "",
		},
	}
)

func init() {
	cobra.OnInitialize(cmd.InitConfig(configViper, configFile, configDir, "config", true))
	cmd.InitConfigCmd(rootCmd, configViper, configDir)

	rootCmd.PersistentFlags().StringVar(
		&configFile,
		"config",
		"",
		"Config file (default ${HOME}/"+configDir+"/config.yml)")

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
		"addrApiProxy",
		flags["addrApiProxy"].DefValue.(string),
		"Textile API proxy listen address")

	rootCmd.PersistentFlags().String(
		"addrThreadsHost",
		flags["addrThreadsHost"].DefValue.(string),
		"Threads peer host listen address")

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

	// Mongo settings
	rootCmd.PersistentFlags().String(
		"addrMongoUri",
		flags["addrMongoUri"].DefValue.(string),
		"MongoDB connection URI")

	// Gateway settings
	rootCmd.PersistentFlags().Bool(
		"gatewaySubdomains",
		flags["gatewaySubdomains"].DefValue.(bool),
		"Enable gateway namespace redirects to subdomains")

	// Cloudflare settings
	rootCmd.PersistentFlags().String(
		"dnsDomain",
		flags["dnsDomain"].DefValue.(string),
		"Root domain for bucket subdomains")

	rootCmd.PersistentFlags().String(
		"dnsZoneID",
		flags["dnsZoneID"].DefValue.(string),
		"Cloudflare ZoneID for dnsDomain")

	rootCmd.PersistentFlags().String(
		"dnsToken",
		flags["dnsDomain"].DefValue.(string),
		"Cloudflare API Token for dnsDomain")

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
	Use:   daemonName,
	Short: "Buckets daemon",
	Long:  `The Buckets daemon.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		configViper.SetConfigType("yaml")
		cmd.ExpandConfigVars(configViper, flags)

		if configViper.GetBool("log.debug") {
			if err := util.SetLogLevels(map[string]logging.LogLevel{
				daemonName: logging.LevelDebug,
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
		addrApiProxy := cmd.AddrFromStr(configViper.GetString("addr.api_proxy"))
		addrThreadsHost := cmd.AddrFromStr(configViper.GetString("addr.threads.host"))
		addrIpfsApi := cmd.AddrFromStr(configViper.GetString("addr.ipfs.api"))

		addrGatewayHost := cmd.AddrFromStr(configViper.GetString("addr.gateway.host"))
		addrGatewayUrl := configViper.GetString("addr.gateway.url")

		var addrPowergateApi ma.Multiaddr
		if str := configViper.GetString("addr.powergate.api"); str != "" {
			addrPowergateApi = cmd.AddrFromStr(str)
		}

		addrMongoUri := configViper.GetString("addr.mongo_uri")

		dnsDomain := configViper.GetString("dns.domain")
		dnsZoneID := configViper.GetString("dns.zone_id")
		dnsToken := configViper.GetString("dns.token")

		logFile := configViper.GetString("log.file")
		if logFile != "" {
			util.SetupDefaultLoggingConfig(logFile)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		textile, err := core.NewTextile(ctx, core.Config{
			RepoPath:         configViper.GetString("repo"),
			AddrAPI:          addrApi,
			AddrAPIProxy:     addrApiProxy,
			AddrThreadsHost:  addrThreadsHost,
			AddrIPFSAPI:      addrIpfsApi,
			AddrGatewayHost:  addrGatewayHost,
			AddrGatewayURL:   addrGatewayUrl,
			AddrPowergateAPI: addrPowergateApi,
			AddrMongoURI:     addrMongoUri,
			UseSubdomains:    configViper.GetBool("gateway.subdomains"),
			MongoName:        "buck",
			DNSDomain:        dnsDomain,
			DNSZoneID:        dnsZoneID,
			DNSToken:         dnsToken,
			Debug:            configViper.GetBool("log.debug"),
		})
		if err != nil {
			log.Fatal(err)
		}
		defer textile.Close()
		textile.Bootstrap()

		fmt.Println("Welcome to Buckets!")
		fmt.Println("Your peer ID is " + textile.HostID().String())

		select {}
	},
}
