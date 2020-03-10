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
		"addrGatewayBucketDomain": {
			Key:      "addr.gateway.bucket_domain",
			DefValue: "textile.cafe",
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
	rootCmd.PersistentFlags().String(
		"addrGatewayBucketDomain",
		flags["addrGatewayBucketDomain"].DefValue.(string),
		"Public gateway bucket domain")

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
		addrApiProxy := cmd.AddrFromStr(configViper.GetString("addr.api_proxy"))
		addrThreadsHost := cmd.AddrFromStr(configViper.GetString("addr.threads.host"))
		addrIpfsApi := cmd.AddrFromStr(configViper.GetString("addr.ipfs.api"))

		addrGatewayHost := cmd.AddrFromStr(configViper.GetString("addr.gateway.host"))
		addrGatewayUrl := configViper.GetString("addr.gateway.url")
		addrGatewayBucketDomain := configViper.GetString("addr.gateway.bucket_domain")

		var addrFilecoinApi ma.Multiaddr
		if str := configViper.GetString("addr.filecoin.api"); str != "" {
			addrFilecoinApi = cmd.AddrFromStr(str)
		}

		addrMongoUri := configViper.GetString("addr.mongo_uri")

		dnsDomain := configViper.GetString("dns.domain")
		dnsZoneID := configViper.GetString("dns.zone_id")
		dnsToken := configViper.GetString("dns.token")

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
			RepoPath:                configViper.GetString("repo"),
			AddrApi:                 addrApi,
			AddrApiProxy:            addrApiProxy,
			AddrThreadsHost:         addrThreadsHost,
			AddrIpfsApi:             addrIpfsApi,
			AddrGatewayHost:         addrGatewayHost,
			AddrGatewayUrl:          addrGatewayUrl,
			AddrGatewayBucketDomain: addrGatewayBucketDomain,
			AddrFilecoinApi:         addrFilecoinApi,
			AddrMongoUri:            addrMongoUri,
			DNSDomain:               dnsDomain,
			DNSZoneID:               dnsZoneID,
			DNSToken:                dnsToken,
			EmailFrom:               emailFrom,
			EmailDomain:             emailDomain,
			EmailApiKey:             emailApiKey,
			Debug:                   configViper.GetBool("log.debug"),
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
