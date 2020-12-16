package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/billingd/service"
	"github.com/textileio/textile/v2/cmd"
)

const daemonName = "billingd"

var (
	log = logging.Logger(daemonName)

	config = &cmd.Config{
		Viper: viper.New(),
		Dir:   "." + daemonName,
		Name:  "config",
		Flags: map[string]cmd.Flag{
			"debug": {
				Key:      "log.debug",
				DefValue: false,
			},
			"logFile": {
				Key:      "log.file",
				DefValue: "", // no log file
			},
			"addrApi": {
				Key:      "addr.api",
				DefValue: "/ip4/127.0.0.1/tcp/10006",
			},
			"addrMongoUri": {
				Key:      "addr.mongo_uri",
				DefValue: "mongodb://127.0.0.1:27017",
			},
			"addrMongoName": {
				Key:      "addr.mongo_name",
				DefValue: "textile_billing",
			},
			"addrGatewayHost": {
				Key:      "addr.gateway.host",
				DefValue: "/ip4/127.0.0.1/tcp/8010",
			},
			"stripeApiUrl": {
				Key:      "stripe.api_url",
				DefValue: "https://api.stripe.com",
			},
			"stripeApiKey": {
				Key:      "stripe.api_key",
				DefValue: "",
			},
			"stripeSessionReturnUrl": {
				Key:      "stripe.session_return_url",
				DefValue: "http://127.0.0.1:8006/dashboard",
			},
			"stripeWebhookSecret": {
				Key:      "stripe.webhook_secret",
				DefValue: "",
			},
			"segmentApiKey": {
				Key:      "segment.api_key",
				DefValue: "",
			},
			"segmentPrefix": {
				Key:      "segment.prefix",
				DefValue: "",
			},
			"freeQuotaGracePeriod": {
				Key:      "free_quota_grace_period",
				DefValue: time.Hour * 24 * 7,
			},
		},
		EnvPre: "BILLING",
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
		"addrMongoUri",
		config.Flags["addrMongoUri"].DefValue.(string),
		"MongoDB connection URI")
	rootCmd.PersistentFlags().String(
		"addrMongoName",
		config.Flags["addrMongoName"].DefValue.(string),
		"MongoDB database name")
	rootCmd.PersistentFlags().String(
		"addrGatewayHost",
		config.Flags["addrGatewayHost"].DefValue.(string),
		"Local gateway host address")

	// Stripe settings
	rootCmd.PersistentFlags().String(
		"stripeApiUrl",
		config.Flags["stripeApiUrl"].DefValue.(string),
		"Stripe API URL")
	rootCmd.PersistentFlags().String(
		"stripeApiKey",
		config.Flags["stripeApiKey"].DefValue.(string),
		"Stripe API secret key")
	rootCmd.PersistentFlags().String(
		"stripeSessionReturnUrl",
		config.Flags["stripeSessionReturnUrl"].DefValue.(string),
		"Stripe portal session return URL")
	rootCmd.PersistentFlags().String(
		"stripeWebhookSecret",
		config.Flags["stripeWebhookSecret"].DefValue.(string),
		"Stripe webhook endpoint secret")

	rootCmd.PersistentFlags().Duration(
		"freeQuotaGracePeriod",
		config.Flags["freeQuotaGracePeriod"].DefValue.(time.Duration),
		"Grace period before blocking usage after free quota is exhausted")

	// Segment settings
	rootCmd.PersistentFlags().String(
		"segmentApiKey",
		config.Flags["segmentApiKey"].DefValue.(string),
		"Segment API key")
	rootCmd.PersistentFlags().String(
		"segmentPrefix",
		config.Flags["segmentPrefix"].DefValue.(string),
		"Segment trait source prefix")

	err := cmd.BindFlags(config.Viper, rootCmd, config.Flags)
	cmd.ErrCheck(err)
}

func main() {
	cmd.ErrCheck(rootCmd.Execute())
}

var rootCmd = &cobra.Command{
	Use:   daemonName,
	Short: "Billing daemon",
	Long:  `Textile's billing daemon.`,
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
		addrMongoUri := config.Viper.GetString("addr.mongo_uri")
		addrMongoName := config.Viper.GetString("addr.mongo_name")

		addrGatewayHost := cmd.AddrFromStr(config.Viper.GetString("addr.gateway.host"))

		stripeApiUrl := config.Viper.GetString("stripe.api_url")
		stripeApiKey := config.Viper.GetString("stripe.api_key")
		stripeSessionReturnUrl := config.Viper.GetString("stripe.session_return_url")
		stripeWebhookSecret := config.Viper.GetString("stripe.webhook_secret")

		freeQuotaGracePeriod := config.Viper.GetDuration("free_quota_grace_period")

		segmentApiKey := config.Viper.GetString("segment.api_key")
		segmentPrefix := config.Viper.GetString("segment.prefix")

		logFile := config.Viper.GetString("log.file")
		if logFile != "" {
			err = cmd.SetupDefaultLoggingConfig(logFile)
			cmd.ErrCheck(err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		api, err := service.NewService(ctx, service.Config{
			ListenAddr:             addrApi,
			StripeAPIURL:           stripeApiUrl,
			StripeAPIKey:           stripeApiKey,
			StripeSessionReturnURL: stripeSessionReturnUrl,
			StripeWebhookSecret:    stripeWebhookSecret,
			SegmentAPIKey:          segmentApiKey,
			SegmentPrefix:          segmentPrefix,
			DBURI:                  addrMongoUri,
			DBName:                 addrMongoName,
			GatewayHostAddr:        addrGatewayHost,
			FreeQuotaGracePeriod:   freeQuotaGracePeriod,
			Debug:                  config.Viper.GetBool("log.debug"),
		})
		cmd.ErrCheck(err)

		err = api.Start()
		cmd.ErrCheck(err)

		fmt.Println("Welcome to Hub Billing!")

		cmd.HandleInterrupt(func() {
			if err := api.Stop(); err != nil {
				fmt.Println(err.Error())
			}
		})
	},
}
