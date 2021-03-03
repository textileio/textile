package main

import (
	"context"
	"encoding/json"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/mindexd/service"
	"github.com/textileio/textile/v2/cmd"
)

const daemonName = "mindexd"

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

			// Addr config
			"addrApi": {
				Key:      "addr.api",
				DefValue: "/ip4/127.0.0.1/tcp/5000",
			},
			"addrApiRest": {
				Key:      "addr.api_rest",
				DefValue: ":8081",
			},
			"addrMongoUri": {
				Key:      "addr.mongo_uri",
				DefValue: "mongodb://127.0.0.1:27017",
			},
			"addrMongoName": {
				Key:      "addr.mongo_name",
				DefValue: "textile_mindex",
			},

			// Powergate config
			"powAddrAPI": {
				Key:      "pow.addr_api",
				DefValue: "",
			},
			"powAdminToken": {
				Key:      "pow.admin_token",
				DefValue: "",
			},

			// Indexer config
			"indexerRunOnStart": {
				Key:      "indexer.run_on_start",
				DefValue: false,
			},
			"indexerFrequency": {
				Key:      "indexer.frequency",
				DefValue: time.Minute * 120,
			},
			"indexerSnapshotMaxAge": {
				Key:      "indexer.snapshot_max_age",
				DefValue: time.Hour * 24,
			},

			// Collector config
			"collectorRunOnStart": {
				Key:      "collector.run_on_start",
				DefValue: false,
			},
			"collectorFrequency": {
				Key:      "collector.frequency",
				DefValue: time.Minute * 60,
			},
			"collectorFetchLimit": {
				Key:      "collector.fetch_limit",
				DefValue: 50,
			},
			"collectorFetchTimeout": {
				Key:      "collector.fetch_timeout",
				DefValue: time.Minute,
			},
		},
		EnvPre: "MINDEX",
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
		"Miner Index gRPC API listen address")
	rootCmd.PersistentFlags().String(
		"addrApiRest",
		config.Flags["addrApiRest"].DefValue.(string),
		"Miner Index REST API listen address")

	rootCmd.PersistentFlags().String(
		"addrMongoUri",
		config.Flags["addrMongoUri"].DefValue.(string),
		"MongoDB connection URI")
	rootCmd.PersistentFlags().String(
		"addrMongoName",
		config.Flags["addrMongoName"].DefValue.(string),
		"MongoDB database name")

	// Powergate settings
	rootCmd.PersistentFlags().String(
		"powAddrAPI",
		config.Flags["powAddrAPI"].DefValue.(string),
		"Powergate API address")
	rootCmd.PersistentFlags().String(
		"powAdminToken",
		config.Flags["powAdminToken"].DefValue.(string),
		"Powergate API admin token")

	// Indexer settings
	rootCmd.PersistentFlags().Bool(
		"indexerRunOnStart",
		config.Flags["indexerRunOnStart"].DefValue.(bool),
		"Indexer run on start")
	rootCmd.PersistentFlags().Duration(
		"indexerFrequency",
		config.Flags["indexerFrequency"].DefValue.(time.Duration),
		"Indexer daemon frequency")
	rootCmd.PersistentFlags().Duration(
		"indexerSnapshotMaxAge",
		config.Flags["indexerSnapshotMaxAge"].DefValue.(time.Duration),
		"Indexer snapshot max-age to trigger a new one.")

	// Collector settings
	rootCmd.PersistentFlags().Bool(
		"collectorRunOnStart",
		config.Flags["collectorRunOnStart"].DefValue.(bool),
		"Collector run on start")
	rootCmd.PersistentFlags().Duration(
		"collectorFrequency",
		config.Flags["collectorFrequency"].DefValue.(time.Duration),
		"Collector daemon frequency")
	rootCmd.PersistentFlags().Int(
		"collectorFetchLimit",
		config.Flags["collectorFetchLimit"].DefValue.(int),
		"Collector biggest batch size for fetching new records")
	rootCmd.PersistentFlags().Duration(
		"collectorFetchTimeout",
		config.Flags["collectorFetchTimeout"].DefValue.(time.Duration),
		"Collector timeout while fetching new records from a target")

	err := cmd.BindFlags(config.Viper, rootCmd, config.Flags)
	cmd.ErrCheck(err)
}

func main() {
	cmd.ErrCheck(rootCmd.Execute())
}

var rootCmd = &cobra.Command{
	Use:   daemonName,
	Short: "Miner Index daemon",
	Long:  `Textile's miner index daemon.`,
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

		grpcListenAddr := cmd.AddrFromStr(config.Viper.GetString("addr.api"))
		restListenAddr := config.Viper.GetString("addr.api_rest")
		addrMongoUri := config.Viper.GetString("addr.mongo_uri")
		addrMongoName := config.Viper.GetString("addr.mongo_name")

		logFile := config.Viper.GetString("log.file")
		if logFile != "" {
			err = cmd.SetupDefaultLoggingConfig(logFile)
			cmd.ErrCheck(err)
		}

		powAddrAPI := config.Viper.GetString("pow.addr_api")
		powAdminToken := config.Viper.GetString("pow.admin_token")

		indexerRunOnStart := config.Viper.GetBool("indexer.run_on_start")
		indexerFrequency := config.Viper.GetDuration("indexer.frequency")
		indexerSnapshotMaxAge := config.Viper.GetDuration("indexer.snapshot_max_age")

		collectorRunOnStart := config.Viper.GetBool("collector.run_on_start")
		collectorFrequency := config.Viper.GetDuration("collector.frequency")
		collectorFetchLimit := config.Viper.GetInt("collector.fetch_limit")
		collectorFetchTimeout := config.Viper.GetDuration("collector.fetch_timeout")
		cmd.ErrCheck(err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		api, err := service.NewService(ctx, service.Config{
			ListenAddrGRPC: grpcListenAddr,
			ListenAddrREST: restListenAddr,
			DBURI:          addrMongoUri,
			DBName:         addrMongoName,
			Debug:          config.Viper.GetBool("log.debug"),

			PowAddrAPI:    powAddrAPI,
			PowAdminToken: powAdminToken,

			CollectorRunOnStart:   collectorRunOnStart,
			CollectorFrequency:    collectorFrequency,
			CollectorFetchLimit:   collectorFetchLimit,
			CollectorFetchTimeout: collectorFetchTimeout,

			IndexerRunOnStart:     indexerRunOnStart,
			IndexerFrequency:      indexerFrequency,
			IndexerSnapshotMaxAge: indexerSnapshotMaxAge,
		})
		cmd.ErrCheck(err)

		err = api.Start()
		cmd.ErrCheck(err)

		log.Info("Welcome to Hub Miner Index!")

		cmd.HandleInterrupt(api.Stop)
	},
}
