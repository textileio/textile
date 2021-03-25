package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	logging "github.com/ipfs/go-log/v2"
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
				DefValue: "", // no log file
			},

			// Addresses
			"addrApi": {
				Key:      "addr.api",
				DefValue: "/ip4/127.0.0.1/tcp/3006",
			},
			"addrApiProxy": {
				Key:      "addr.api_proxy",
				DefValue: "/ip4/127.0.0.1/tcp/3007",
			},
			"addrMongoUri": {
				Key:      "addr.mongo_uri",
				DefValue: "mongodb://127.0.0.1:27017",
			},
			"addrMongoName": {
				Key:      "addr.mongo_name",
				DefValue: "buckets",
			},
			"addrThreadsHost": {
				Key:      "addr.threads.host",
				DefValue: "/ip4/0.0.0.0/tcp/4006",
			},
			"addrThreadsMongoUri": {
				Key:      "addr.threads.mongo_uri",
				DefValue: "",
			},
			"addrThreadsMongoName": {
				Key:      "addr.threads.mongo_name",
				DefValue: "",
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

			// IPNS
			"ipnsRepublishSchedule": {
				Key:      "ipns.republish_schedule",
				DefValue: "0 1 * * *",
			},
			"ipnsRepublishConcurrency": {
				Key:      "ipns.republish_concurrency",
				DefValue: 100,
			},

			// Gateway
			"gatewaySubdomains": {
				Key:      "gateway.subdomains",
				DefValue: false,
			},

			// Cloudflare
			// @todo: Change these to cloudflareDnsDomain, etc.
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

	// Addresses
	rootCmd.PersistentFlags().String(
		"addrApi",
		config.Flags["addrApi"].DefValue.(string),
		"Hub API listen address")
	rootCmd.PersistentFlags().String(
		"addrApiProxy",
		config.Flags["addrApiProxy"].DefValue.(string),
		"Hub API proxy listen address")
	rootCmd.PersistentFlags().String(
		"addrMongoUri",
		config.Flags["addrMongoUri"].DefValue.(string),
		"MongoDB connection URI")
	rootCmd.PersistentFlags().String(
		"addrMongoName",
		config.Flags["addrMongoName"].DefValue.(string),
		"MongoDB database name")
	rootCmd.PersistentFlags().String(
		"addrThreadsHost",
		config.Flags["addrThreadsHost"].DefValue.(string),
		"Threads peer host listen address")
	rootCmd.PersistentFlags().String(
		"addrThreadsMongoUri",
		config.Flags["addrThreadsMongoUri"].DefValue.(string),
		"Threads MongoDB connection URI")
	rootCmd.PersistentFlags().String(
		"addrThreadsMongoName",
		config.Flags["addrThreadsMongoName"].DefValue.(string),
		"Threads MongoDB database name")
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

	// IPNS
	rootCmd.PersistentFlags().String(
		"ipnsRepublishSchedule",
		config.Flags["ipnsRepublishSchedule"].DefValue.(string),
		"IPNS republishing cron schedule")
	rootCmd.PersistentFlags().Int(
		"ipnsRepublishConcurrency",
		config.Flags["ipnsRepublishConcurrency"].DefValue.(int),
		"IPNS republishing batch size")

	// Gateway
	rootCmd.PersistentFlags().Bool(
		"gatewaySubdomains",
		config.Flags["gatewaySubdomains"].DefValue.(bool),
		"Enable gateway namespace redirects to subdomains")

	// Cloudflare
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
		config.Flags["dnsToken"].DefValue.(string),
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

		debug := config.Viper.GetBool("log.debug")
		logFile := config.Viper.GetString("log.file")
		err = cmd.SetupDefaultLoggingConfig(logFile)
		cmd.ErrCheck(err)

		addrApi := cmd.AddrFromStr(config.Viper.GetString("addr.api"))
		addrApiProxy := cmd.AddrFromStr(config.Viper.GetString("addr.api_proxy"))
		addrMongoUri := config.Viper.GetString("addr.mongo_uri")
		addrMongoName := config.Viper.GetString("addr.mongo_name")
		addrThreadsHost := cmd.AddrFromStr(config.Viper.GetString("addr.threads.host"))
		addrThreadsMongoUri := config.Viper.GetString("addr.threads.mongo_uri")
		addrThreadsMongoName := config.Viper.GetString("addr.threads.mongo_name")
		ipnsRepublishSchedule := config.Viper.GetString("ipns.republish_schedule")
		ipnsRepublishConcurrency := config.Viper.GetInt("ipns.republish_concurrency")
		addrGatewayHost := cmd.AddrFromStr(config.Viper.GetString("addr.gateway.host"))
		addrGatewayUrl := config.Viper.GetString("addr.gateway.url")
		addrIpfsApi := cmd.AddrFromStr(config.Viper.GetString("addr.ipfs.api"))
		addrPowergateApi := config.Viper.GetString("addr.powergate.api")

		dnsDomain := config.Viper.GetString("dns.domain")
		dnsZoneID := config.Viper.GetString("dns.zone_id")
		dnsToken := config.Viper.GetString("dns.token")

		var opts []core.Option
		if addrThreadsMongoUri != "" {
			if addrThreadsMongoName == "" {
				cmd.Fatal(errors.New("addr.threads.mongo_name is required with addr.threads.mongo_uri"))
			}
			opts = append(opts, core.WithMongoThreadsPersistence(addrThreadsMongoUri, addrThreadsMongoName))
		} else {
			opts = append(opts, core.WithBadgerThreadsPersistence(config.Viper.GetString("repo")))
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		textile, err := core.NewTextile(ctx, core.Config{
			Debug: debug,

			AddrAPI:                  addrApi,
			AddrAPIProxy:             addrApiProxy,
			AddrMongoURI:             addrMongoUri,
			AddrMongoName:            addrMongoName,
			AddrThreadsHost:          addrThreadsHost,
			AddrGatewayHost:          addrGatewayHost,
			AddrGatewayURL:           addrGatewayUrl,
			AddrIPFSAPI:              addrIpfsApi,
			AddrPowergateAPI:         addrPowergateApi,
			IPNSRepublishSchedule:    ipnsRepublishSchedule,
			IPNSRepublishConcurrency: ipnsRepublishConcurrency,
			UseSubdomains:            config.Viper.GetBool("gateway.subdomains"),

			DNSDomain: dnsDomain,
			DNSZoneID: dnsZoneID,
			DNSToken:  dnsToken,
		}, opts...)
		cmd.ErrCheck(err)
		textile.Bootstrap()

		fmt.Println("Welcome to Buckets!")
		fmt.Println("Your peer ID is " + textile.HostID().String())

		cmd.HandleInterrupt(func() {
			if err := textile.Close(); err != nil {
				fmt.Println(err.Error())
			}
		})
	},
}
