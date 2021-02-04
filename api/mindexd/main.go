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
	"github.com/textileio/textile/v2/api/mindexd/collector"
	"github.com/textileio/textile/v2/api/mindexd/service"
	"github.com/textileio/textile/v2/cmd"

	"golang.org/x/text/language"
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
				DefValue: "/ip4/127.0.0.1/tcp/20006",
			},
			"addrMongoUri": {
				Key:      "addr.mongo_uri",
				DefValue: "mongodb://127.0.0.1:27017",
			},
			"addrMongoName": {
				Key:      "addr.mongo_name",
				DefValue: "textile_mindex",
			},

			// Collector config
			"collectorRunOnStartup": {
				Key:      "collector.run_on_startup",
				DefValue: false,
			},
			"collectorFrequency": {
				Key:      "collector.frequency",
				DefValue: time.Minute * 60,
			},
			"collectorTargets": {
				Key:      "collector.targets",
				DefValue: "",
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
		"Hub API listen address")
	rootCmd.PersistentFlags().String(
		"addrMongoUri",
		config.Flags["addrMongoUri"].DefValue.(string),
		"MongoDB connection URI")
	rootCmd.PersistentFlags().String(
		"addrMongoName",
		config.Flags["addrMongoName"].DefValue.(string),
		"MongoDB database name")

	// Collector settings
	rootCmd.PersistentFlags().Bool(
		"collectorRunOnStart",
		config.Flags["collectorRunOnStart"].DefValue.(bool),
		"Collector run on start")
	rootCmd.PersistentFlags().Duration(
		"collectorFrequency",
		config.Flags["collectorFrequency"].DefValue.(time.Duration),
		"Collector daemon frequency")
	rootCmd.PersistentFlags().String(
		"collectorTargets",
		config.Flags["collectorTargets"].DefValue.(string),
		"Collector targets (JSON)")
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

		addrApi := cmd.AddrFromStr(config.Viper.GetString("addr.api"))
		addrMongoUri := config.Viper.GetString("addr.mongo_uri")
		addrMongoName := config.Viper.GetString("addr.mongo_name")

		logFile := config.Viper.GetString("log.file")
		if logFile != "" {
			err = cmd.SetupDefaultLoggingConfig(logFile)
			cmd.ErrCheck(err)
		}

		collectorRunOnStart := config.Viper.GetBool("collector.run_on_start")
		collectorFrequency := config.Viper.GetDuration("collector.frequency")
		collectorFetchLimit := config.Viper.GetInt("collector.fetch_limit")
		collectorFetchTimeout := config.Viper.GetDuration("collector.fetch_duration")
		collectorTargets, err := parseCollectorTargets(config.Viper.GetString("collector.targets"))
		cmd.ErrCheck(err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		api, err := service.NewService(ctx, service.Config{
			ListenAddr: addrApi,
			DBURI:      addrMongoUri,
			DBName:     addrMongoName,
			Debug:      config.Viper.GetBool("log.debug"),

			CollectorRunOnStart:   collectorRunOnStart,
			CollectorFrequency:    collectorFrequency,
			CollectorTargets:      collectorTargets,
			CollectorFetchLimit:   collectorFetchLimit,
			CollectorFetchTimeout: collectorFetchTimeout,
		})
		cmd.ErrCheck(err)

		err = api.Start()
		cmd.ErrCheck(err)

		fmt.Println("Welcome to Hub Miner Index!")

		cmd.HandleInterrupt(api.Stop)
	},
}

type targetsJSON struct {
	Targets []collector.PowTarget `json:"targets"`
}

func parseCollectorTargets(targetsEncoded string) ([]collector.PowTarget, error) {
	var targets targetsJSON
	if err := json.Unmarshal([]byte(targetsEncoded), &targets); err != nil {
		return nil, fmt.Errorf("unmarshaling config targets: %s", err)
	}

	// Validate if regions are valid M49 codes.
	for _, t := range targets.Targets {
		if _, err := language.ParseRegion(t.Region); err != nil {
			return nil, fmt.Errorf("region %s isn't a valid M49 code: %s", t.Region, err)
		}
	}

	return targets.Targets, nil
}
