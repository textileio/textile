package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/cmd"
	"github.com/textileio/textile/v2/core"
)

const (
	daemonName = "hubd"

	mib = 1024 * 1024
	gib = 1024 * mib
)

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
				DefValue: "textile",
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
			"addrBillingApi": {
				Key:      "addr.billing.api",
				DefValue: "",
			},
			"addrPowergateApi": {
				Key:      "addr.powergate.api",
				DefValue: "",
			},

			// Buckets
			"bucketsArchiveMaxRepFactor": {
				Key:      "buckets.archive_max_rep_factor",
				DefValue: 4,
			},
			"bucketsArchiveMaxSize": {
				Key:      "buckets.archive_max_size",
				DefValue: int64(32 * gib),
			},
			"bucketsArchiveMinSize": {
				Key:      "buckets.archive_min_size",
				DefValue: int64(64 * mib),
			},

			// Threads
			"threadsMaxNumberPerOwner": {
				Key:      "threads.max_number_per_owner",
				DefValue: 100,
			},

			// Powergate
			"powergateAdminToken": {
				Key:      "powergate.admin_token",
				DefValue: "",
			},

			// Archives
			"archivesJobPollIntervalSlow": {
				Key:      "archives.job_poll_interval_slow",
				DefValue: time.Minute * 30,
			},
			"archivesJobPollIntervalFast": {
				Key:      "archives.job_poll_interval_fast",
				DefValue: time.Minute * 15,
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

			// Customer.io
			"customerioApiKey": {
				Key:      "customerio.api_key",
				DefValue: "",
			},
			"customerioInviteTmpl": {
				Key:      "customerio.invite_template",
				DefValue: "2",
			},
			"customerioConfirmTmpl": {
				Key:      "customerio.confirm_template",
				DefValue: "3",
			},
			"emailSessionSecret": {
				Key:      "email.session_secret",
				DefValue: "",
			},
		},
		EnvPre: "HUB",
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
		"addrBillingApi",
		config.Flags["addrBillingApi"].DefValue.(string),
		"Billing API address")
	rootCmd.PersistentFlags().String(
		"addrPowergateApi",
		config.Flags["addrPowergateApi"].DefValue.(string),
		"Powergate API address")

	// Buckets
	rootCmd.PersistentFlags().Int(
		"bucketsArchiveMaxRepFactor",
		config.Flags["bucketsArchiveMaxRepFactor"].DefValue.(int),
		"Bucket archive max replication factor")
	rootCmd.PersistentFlags().Int64(
		"bucketsArchiveMaxSize",
		config.Flags["bucketsArchiveMaxSize"].DefValue.(int64),
		"Bucket archive max size")
	rootCmd.PersistentFlags().Int64(
		"bucketsArchiveMinSize",
		config.Flags["bucketsArchiveMinSize"].DefValue.(int64),
		"Bucket archive min size")

	// Threads
	rootCmd.PersistentFlags().Int(
		"threadsMaxNumberPerOwner",
		config.Flags["threadsMaxNumberPerOwner"].DefValue.(int),
		"Max number threads per owner")

	// Powergate
	rootCmd.PersistentFlags().String(
		"powergateAdminToken",
		config.Flags["powergateAdminToken"].DefValue.(string),
		"Auth token for Powergate admin APIs")

	// Archives
	// @todo: Move these under the powergate namespace
	rootCmd.PersistentFlags().Duration(
		"archivesJobPollIntervalSlow",
		config.Flags["archivesJobPollIntervalSlow"].DefValue.(time.Duration),
		"How frequently to check archive job status for arcives with deals in the sealing state")
	rootCmd.PersistentFlags().Duration(
		"archivesJobPollIntervalFast",
		config.Flags["archivesJobPollIntervalFast"].DefValue.(time.Duration),
		"How frequently to check archive job status for arcives with deals in non-sealing states")

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
	// @todo: Change these to cloudflareDnsDomain, etc.
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

	// Customer.io
	rootCmd.PersistentFlags().String(
		"customerioApiKey",
		config.Flags["customerioApiKey"].DefValue.(string),
		"Customer.io API key for sending emails")
	rootCmd.PersistentFlags().String(
		"customerioConfirmTmpl",
		config.Flags["customerioConfirmTmpl"].DefValue.(string),
		"Template ID for confirmation emails")
	rootCmd.PersistentFlags().String(
		"customerioInviteTmpl",
		config.Flags["customerioInviteTmpl"].DefValue.(string),
		"Template ID for invite emails")
	// @todo: Change this to the customerio namespace
	rootCmd.PersistentFlags().String(
		"emailSessionSecret",
		config.Flags["emailSessionSecret"].DefValue.(string),
		"Session secret to use when testing email APIs")

	err := cmd.BindFlags(config.Viper, rootCmd, config.Flags)
	cmd.ErrCheck(err)
}

func main() {
	cmd.ErrCheck(rootCmd.Execute())
}

var rootCmd = &cobra.Command{
	Use:   daemonName,
	Short: "Hub daemon",
	Long:  `The Hub daemon.`,
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
		if logFile != "" {
			err = cmd.SetupDefaultLoggingConfig(logFile)
			cmd.ErrCheck(err)
		}

		// Addresses
		addrApi := cmd.AddrFromStr(config.Viper.GetString("addr.api"))
		addrApiProxy := cmd.AddrFromStr(config.Viper.GetString("addr.api_proxy"))
		addrMongoUri := config.Viper.GetString("addr.mongo_uri")
		addrMongoName := config.Viper.GetString("addr.mongo_name")
		addrThreadsHost := cmd.AddrFromStr(config.Viper.GetString("addr.threads.host"))
		addrThreadsMongoUri := config.Viper.GetString("addr.threads.mongo_uri")
		addrThreadsMongoName := config.Viper.GetString("addr.threads.mongo_name")
		addrGatewayHost := cmd.AddrFromStr(config.Viper.GetString("addr.gateway.host"))
		addrGatewayUrl := config.Viper.GetString("addr.gateway.url")
		addrIpfsApi := cmd.AddrFromStr(config.Viper.GetString("addr.ipfs.api"))
		addrBillingApi := config.Viper.GetString("addr.billing.api")
		addrPowergateApi := config.Viper.GetString("addr.powergate.api")

		// Buckets
		bucketsArchiveMaxRepFactor := config.Viper.GetInt("buckets.archive_max_rep_factor")
		bucketsArchiveMaxSize := config.Viper.GetInt64("buckets.archive_max_size")
		bucketsArchiveMinSize := config.Viper.GetInt64("buckets.archive_min_size")

		// Threads
		threadsMaxNumberPerOwner := config.Viper.GetInt("threads.max_number_per_owner")

		// Powergate
		powergateAdminToken := config.Viper.GetString("powergate.admin_token")

		// Archives
		archivesJobPollIntervalSlow := config.Viper.GetDuration("archives.job_poll_interval_slow")
		archivesJobPollIntervalFast := config.Viper.GetDuration("archives.job_poll_interval_fast")

		// IPNS
		ipnsRepublishSchedule := config.Viper.GetString("ipns.republish_schedule")
		ipnsRepublishConcurrency := config.Viper.GetInt("ipns.republish_concurrency")

		// Gateway
		gatewaySubdomains := config.Viper.GetBool("gateway.subdomains")

		// Cloudflare
		dnsDomain := config.Viper.GetString("dns.domain")
		dnsZoneID := config.Viper.GetString("dns.zone_id")
		dnsToken := config.Viper.GetString("dns.token")

		// Customer.io
		customerioApiKey := config.Viper.GetString("customerio.api_key")
		customerioConfirmTmpl := config.Viper.GetString("customerio.confirm_template")
		customerioInviteTmpl := config.Viper.GetString("customerio.invite_template")
		emailSessionSecret := config.Viper.GetString("email.session_secret")

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
			Hub:   true,
			Debug: debug,
			// Addresses
			AddrAPI:          addrApi,
			AddrAPIProxy:     addrApiProxy,
			AddrMongoURI:     addrMongoUri,
			AddrMongoName:    addrMongoName,
			AddrThreadsHost:  addrThreadsHost,
			AddrGatewayHost:  addrGatewayHost,
			AddrGatewayURL:   addrGatewayUrl,
			AddrIPFSAPI:      addrIpfsApi,
			AddrBillingAPI:   addrBillingApi,
			AddrPowergateAPI: addrPowergateApi,
			// Buckets
			MaxBucketArchiveRepFactor: bucketsArchiveMaxRepFactor,
			MaxBucketArchiveSize:      bucketsArchiveMaxSize,
			MinBucketArchiveSize:      bucketsArchiveMinSize,
			// Threads
			MaxNumberThreadsPerOwner: threadsMaxNumberPerOwner,
			// Powergate
			PowergateAdminToken: powergateAdminToken,
			// Archives
			ArchiveJobPollIntervalSlow: archivesJobPollIntervalSlow,
			ArchiveJobPollIntervalFast: archivesJobPollIntervalFast,
			// IPNS
			IPNSRepublishSchedule:    ipnsRepublishSchedule,
			IPNSRepublishConcurrency: ipnsRepublishConcurrency,
			// Gateway
			UseSubdomains: gatewaySubdomains,
			// Cloudflare
			DNSDomain: dnsDomain,
			DNSZoneID: dnsZoneID,
			DNSToken:  dnsToken,
			// Customer.io
			CustomerioConfirmTmpl: customerioConfirmTmpl,
			CustomerioInviteTmpl:  customerioInviteTmpl,
			CustomerioAPIKey:      customerioApiKey,
			EmailSessionSecret:    emailSessionSecret,
		}, opts...)
		cmd.ErrCheck(err)
		textile.Bootstrap()

		fmt.Println("Welcome to the Hub!")
		fmt.Println("Your peer ID is " + textile.HostID().String())

		cmd.HandleInterrupt(func() {
			if err := textile.Close(); err != nil {
				fmt.Println(err.Error())
			}
		})
	},
}
