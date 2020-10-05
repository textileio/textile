package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"

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
				DefValue: "mongodb://127.0.0.1:27017/?replicaSet=rs0",
			},
			"addrMongoName": {
				Key:      "addr.mongo_name",
				DefValue: "hub_billing",
			},
			"stripeKey": {
				Key:      "stripe.key",
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

	// Stripe settings
	rootCmd.PersistentFlags().String(
		"stripeKey",
		config.Flags["stripeKey"].DefValue.(string),
		"Stripe secret key")
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

		stripeKey := config.Viper.GetString("stripe.key")
		stripeStoredDataPrice := config.Viper.GetString("stripe.stored_data.price")
		stripeNetworkEgressPrice := config.Viper.GetString("stripe.network_egress.price")
		stripeInstanceReadsPrice := config.Viper.GetString("stripe.instance_reads.price")
		stripeInstanceWritesPrice := config.Viper.GetString("stripe.instance_writes.price")

		logFile := config.Viper.GetString("log.file")
		if logFile != "" {
			err = util.SetupDefaultLoggingConfig(logFile)
			cmd.ErrCheck(err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		api, err := service.NewService(ctx, service.Config{
			ListenAddr:            addrApi,
			StripeKey:             stripeKey,
			DBURI:                 addrMongoUri,
			DBName:                addrMongoName,
			StoredDataPriceID:     stripeStoredDataPrice,
			NetworkEgressPriceID:  stripeNetworkEgressPrice,
			InstanceReadsPriceID:  stripeInstanceReadsPrice,
			InstanceWritesPriceID: stripeInstanceWritesPrice,
			Debug: config.Viper.GetBool("log.debug"),
		}, false)
		cmd.ErrCheck(err)

		err = api.Start()
		cmd.ErrCheck(err)

		fmt.Println("Hub billing started.")

		quit := make(chan os.Signal)
		signal.Notify(quit, os.Interrupt)
		<-quit
		fmt.Println("Gracefully stopping... (press Ctrl+C again to force)")
		err = api.Stop(false)
		if err != nil {
			fmt.Println(err.Error())
		}
		os.Exit(1)
	},
}
