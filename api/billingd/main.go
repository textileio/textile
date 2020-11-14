package main

import (
	"context"
	"encoding/json"
	"fmt"

	logging "github.com/ipfs/go-log"
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
				DefValue: "${HOME}/." + daemonName + "/log",
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
			"stripeStoredDataPrice": {
				Key:      "stripe.stored_data.price",
				DefValue: "",
			},
			"stripeNetworkEgressPrice": {
				Key:      "stripe.network_egress.price",
				DefValue: "",
			},
			"stripeInstanceReadsPrice": {
				Key:      "stripe.instance_reads.price",
				DefValue: "",
			},
			"stripeInstanceWritesPrice": {
				Key:      "stripe.instance_writes.price",
				DefValue: "",
			},
			"stripeStoredDataDependentPrice": {
				Key:      "stripe.stored_data.dependent_price",
				DefValue: "",
			},
			"stripeNetworkEgressDependentPrice": {
				Key:      "stripe.network_egress.dependent_price",
				DefValue: "",
			},
			"stripeInstanceReadsDependentPrice": {
				Key:      "stripe.instance_reads.dependent_price",
				DefValue: "",
			},
			"stripeInstanceWritesDependentPrice": {
				Key:      "stripe.instance_writes.dependent_price",
				DefValue: "",
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
	rootCmd.PersistentFlags().String(
		"stripeStoredDataPrice",
		config.Flags["stripeStoredDataPrice"].DefValue.(string),
		"Stripe price ID for stored data")
	rootCmd.PersistentFlags().String(
		"stripeNetworkEgressPrice",
		config.Flags["stripeNetworkEgressPrice"].DefValue.(string),
		"Stripe price ID for network egress")
	rootCmd.PersistentFlags().String(
		"stripeInstanceReadsPrice",
		config.Flags["stripeInstanceReadsPrice"].DefValue.(string),
		"Stripe price ID for instance reads")
	rootCmd.PersistentFlags().String(
		"stripeInstanceWritesPrice",
		config.Flags["stripeInstanceWritesPrice"].DefValue.(string),
		"Stripe price ID for instance writes")
	rootCmd.PersistentFlags().String(
		"stripeStoredDataDependentPrice",
		config.Flags["stripeStoredDataDependentPrice"].DefValue.(string),
		"Stripe price ID for dependent users stored data")
	rootCmd.PersistentFlags().String(
		"stripeNetworkEgressDependentPrice",
		config.Flags["stripeNetworkEgressDependentPrice"].DefValue.(string),
		"Stripe price ID for dependent users network egress")
	rootCmd.PersistentFlags().String(
		"stripeInstanceReadsDependentPrice",
		config.Flags["stripeInstanceReadsDependentPrice"].DefValue.(string),
		"Stripe price ID for dependent users instance reads")
	rootCmd.PersistentFlags().String(
		"stripeInstanceWritesDependentPrice",
		config.Flags["stripeInstanceWritesDependentPrice"].DefValue.(string),
		"Stripe price ID for dependent users instance writes")

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
		stripeStoredDataPrice := config.Viper.GetString("stripe.stored_data.price")
		stripeNetworkEgressPrice := config.Viper.GetString("stripe.network_egress.price")
		stripeInstanceReadsPrice := config.Viper.GetString("stripe.instance_reads.price")
		stripeInstanceWritesPrice := config.Viper.GetString("stripe.instance_writes.price")
		stripeStoredDataDependentPrice := config.Viper.GetString("stripe.stored_data.dependent_price")
		stripeNetworkEgressDependentPrice := config.Viper.GetString("stripe.network_egress.dependent_price")
		stripeInstanceReadsDependentPrice := config.Viper.GetString("stripe.instance_reads.dependent_price")
		stripeInstanceWritesDependentPrice := config.Viper.GetString("stripe.instance_writes.dependent_price")

		logFile := config.Viper.GetString("log.file")
		if logFile != "" {
			err = util.SetupDefaultLoggingConfig(logFile)
			cmd.ErrCheck(err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		api, err := service.NewService(ctx, service.Config{
			ListenAddr:                addrApi,
			StripeAPIURL:              stripeApiUrl,
			StripeAPIKey:              stripeApiKey,
			StripeSessionReturnURL:    stripeSessionReturnUrl,
			StripeWebhookSecret:       stripeWebhookSecret,
			DBURI:                     addrMongoUri,
			DBName:                    addrMongoName,
			GatewayHostAddr:           addrGatewayHost,
			StoredDataFreePriceID:     stripeStoredDataPrice,
			NetworkEgressFreePriceID:  stripeNetworkEgressPrice,
			InstanceReadsFreePriceID:  stripeInstanceReadsPrice,
			InstanceWritesFreePriceID: stripeInstanceWritesPrice,
			StoredDataPaidPriceID:     stripeStoredDataDependentPrice,
			NetworkEgressPaidPriceID:  stripeNetworkEgressDependentPrice,
			InstanceReadsPaidPriceID:  stripeInstanceReadsDependentPrice,
			InstanceWritesPaidPriceID: stripeInstanceWritesDependentPrice,
			Debug: config.Viper.GetBool("log.debug"),
		})
		cmd.ErrCheck(err)

		err = api.Start()
		cmd.ErrCheck(err)

		fmt.Println("Welcome to Hub Billing!")

		cmd.HandleInterrupt(func() {
			if err := api.Stop(false); err != nil {
				fmt.Println(err.Error())
			}
		})
	},
}
