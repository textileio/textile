package main

import (
	"context"
	"encoding/json"
	"fmt"

	logging "github.com/ipfs/go-log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/cmd"
	"github.com/textileio/textile/v2/core"
)

const daemonName = "buckd"

var (
	log = logging.Logger(daemonName)

	config = &cmd.Config{
		Viper: viper.New(),
		Dir:   "." + daemonName,
		Name:  "config",
		Flags: map[string]cmd.Flag{
			"repo": {
				Key:      "repo",
				DefValue: "${HOME}/." + daemonName + "/repo",
			},
			"debug": {
				Key:      "log.debug",
				DefValue: false,
			},
			"logFile": {
				Key:      "log.file",
				DefValue: "${HOME}/." + daemonName + "/log",
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
			"addrPowergateApi": {
				Key:      "addr.powergate.api",
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
		},
		EnvPre: "BUCK",
		Global: true,
	}
)

func init() {
	cobra.OnInitialize(cmd.InitConfig(config))
	cmd.InitConfigCmd(rootCmd, config.Viper, config.Dir)

	rootCmd.PersistentFlags().StringVar(
		&config.File,
		"config",
		"",
		"Config file (default ${HOME}/"+config.Dir+"/"+config.Name+".yml)")
	rootCmd.PersistentFlags().StringP(
		"repo",
		"r",
		config.Flags["repo"].DefValue.(string),
		"Path to repository")
	rootCmd.PersistentFlags().BoolP(
		"debug",
		"d",
		config.Flags["debug"].DefValue.(bool),
		"Enable debug logging")
	rootCmd.PersistentFlags().String(
		"logFile",
		config.Flags["logFile"].DefValue.(string),
		"Write logs to file")

	// Address settings
	rootCmd.PersistentFlags().String(
		"addrApi",
		config.Flags["addrApi"].DefValue.(string),
		"Hub API listen address")
	rootCmd.PersistentFlags().String(
		"addrApiProxy",
		config.Flags["addrApiProxy"].DefValue.(string),
		"Hub API proxy listen address")
	rootCmd.PersistentFlags().String(
		"addrThreadsHost",
		config.Flags["addrThreadsHost"].DefValue.(string),
		"Threads peer host listen address")
	rootCmd.PersistentFlags().String(
		"addrGatewayHost",
		config.Flags["addrGatewayHost"].DefValue.(string),
		"Local gateway host address")
	rootCmd.PersistentFlags().String(
		"addrGatewayUrl",
		config.Flags["addrGatewayUrl"].DefValue.(string),
		"Public gateway address")
	rootCmd.PersistentFlags().String(
		"addrIpfsApi",
		config.Flags["addrIpfsApi"].DefValue.(string),
		"IPFS API address")
	rootCmd.PersistentFlags().String(
		"addrPowergateApi",
		config.Flags["addrPowergateApi"].DefValue.(string),
		"Powergate API address")
	rootCmd.PersistentFlags().String(
		"addrMongoUri",
		config.Flags["addrMongoUri"].DefValue.(string),
		"MongoDB connection URI")

	// Gateway settings
	rootCmd.PersistentFlags().Bool(
		"gatewaySubdomains",
		config.Flags["gatewaySubdomains"].DefValue.(bool),
		"Enable gateway namespace redirects to subdomains")

	// DNS settings
	rootCmd.PersistentFlags().String(
		"dnsDomain",
		config.Flags["dnsDomain"].DefValue.(string),
		"Root domain for bucket subdomains")
	rootCmd.PersistentFlags().String(
		"dnsZoneID",
		config.Flags["dnsZoneID"].DefValue.(string),
		"Cloudflare ZoneID for dnsDomain")
	rootCmd.PersistentFlags().String(
		"dnsToken",
		config.Flags["dnsDomain"].DefValue.(string),
		"Cloudflare API Token for dnsDomain")

	err := cmd.BindFlags(config.Viper, rootCmd, config.Flags)
	cmd.ErrCheck(err)
}

func main() {
	cmd.ErrCheck(rootCmd.Execute())
}

var rootCmd = &cobra.Command{
	Use:   daemonName,
	Short: "Buckets daemon",
	Long:  `The Buckets daemon.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		config.Viper.SetConfigType("yaml")
		cmd.ExpandConfigVars(config.Viper, config.Flags)

		if config.Viper.GetBool("log.debug") {
			err := util.SetLogLevels(map[string]logging.LogLevel{
				daemonName: logging.LevelDebug,
			})
			cmd.ErrCheck(err)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		settings, err := json.MarshalIndent(config.Viper.AllSettings(), "", "  ")
		cmd.ErrCheck(err)
		log.Debugf("loaded config: %s", string(settings))

		addrApi := cmd.AddrFromStr(config.Viper.GetString("addr.api"))
		addrApiProxy := cmd.AddrFromStr(config.Viper.GetString("addr.api_proxy"))
		addrThreadsHost := cmd.AddrFromStr(config.Viper.GetString("addr.threads.host"))
		addrIpfsApi := cmd.AddrFromStr(config.Viper.GetString("addr.ipfs.api"))

		addrPowergateApi := config.Viper.GetString("addr.powergate.api")

		addrGatewayHost := cmd.AddrFromStr(config.Viper.GetString("addr.gateway.host"))
		addrGatewayUrl := config.Viper.GetString("addr.gateway.url")

		addrMongoUri := config.Viper.GetString("addr.mongo_uri")

		dnsDomain := config.Viper.GetString("dns.domain")
		dnsZoneID := config.Viper.GetString("dns.zone_id")
		dnsToken := config.Viper.GetString("dns.token")

		logFile := config.Viper.GetString("log.file")
		if logFile != "" {
			err = util.SetupDefaultLoggingConfig(logFile)
			cmd.ErrCheck(err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		textile, err := core.NewTextile(ctx, core.Config{
			RepoPath: config.Viper.GetString("repo"),

			AddrAPI:          addrApi,
			AddrAPIProxy:     addrApiProxy,
			AddrThreadsHost:  addrThreadsHost,
			AddrIPFSAPI:      addrIpfsApi,
			AddrGatewayHost:  addrGatewayHost,
			AddrGatewayURL:   addrGatewayUrl,
			AddrPowergateAPI: addrPowergateApi,
			AddrMongoURI:     addrMongoUri,

			UseSubdomains: config.Viper.GetBool("gateway.subdomains"),

			MongoName: "buckets",

			DNSDomain: dnsDomain,
			DNSZoneID: dnsZoneID,
			DNSToken:  dnsToken,

			Debug: config.Viper.GetBool("log.debug"),
		})
		cmd.ErrCheck(err)
		defer textile.Close(false)
		textile.Bootstrap()

		fmt.Println("Welcome to Buckets!")
		fmt.Println("Your peer ID is " + textile.HostID().String())

		select {}
	},
}
