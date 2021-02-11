package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/filrewardsd/service"
	"github.com/textileio/textile/v2/cmd"
)

const daemonName = "filrewardsd"

var (
	log = logging.Logger(daemonName)

	config = &cmd.Config{
		Viper: viper.New(),
		Dir:   "." + daemonName,
		Name:  "config",
		Flags: map[string]cmd.Flag{
			"debug": {
				Key:      "debug",
				DefValue: false,
			},
			"logFile": {
				Key:      "log_file",
				DefValue: "", // no log file
			},
			"listenAddr": {
				Key:      "listen_addr",
				DefValue: "/ip4/127.0.0.1/tcp/7006",
			},
			"mongoUri": {
				Key:      "mongo_uri",
				DefValue: "mongodb://127.0.0.1:27017",
			},
			"mongoDb": {
				Key:      "mongo_db",
				DefValue: "textile_filrewards",
			},
			"analyticsAddr": {
				Key:      "analytics_addr",
				DefValue: "",
			},
			"baseAttoFilReward": {
				Key:      "base_atto_fil_reward",
				DefValue: int64(0),
			},
		},
		EnvPre: "FILREWARDS",
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

	rootCmd.PersistentFlags().String(
		"listenAddr",
		config.Flags["listenAddr"].DefValue.(string),
		"Filrewards API listen address")

	rootCmd.PersistentFlags().String(
		"mongoUri",
		config.Flags["mongoUri"].DefValue.(string),
		"MongoDB connection URI")
	rootCmd.PersistentFlags().String(
		"mongoDb",
		config.Flags["mongoDb"].DefValue.(string),
		"MongoDB database name")

	rootCmd.PersistentFlags().String(
		"analyticsAddr",
		config.Flags["analyticsAddr"].DefValue.(string),
		"Analytics API address")

	rootCmd.PersistentFlags().Int64(
		"baseAttoFilReward",
		config.Flags["baseAttoFilReward"].DefValue.(int64),
		"Base AttoFIL reward amount",
	)

	err := cmd.BindFlags(config.Viper, rootCmd, config.Flags)
	cmd.ErrCheck(err)
}

func main() {
	cmd.ErrCheck(rootCmd.Execute())
}

var rootCmd = &cobra.Command{
	Use:   daemonName,
	Short: "Filrewards daemon",
	Long:  `Textile's filrewards daemon.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		config.Viper.SetConfigType("yaml")
		cmd.ExpandConfigVars(config.Viper, config.Flags)

		if config.Viper.GetBool("debug") {
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

		debug := config.Viper.GetBool("debug")
		logFile := config.Viper.GetString("log_file")
		listenAddr := cmd.AddrFromStr(config.Viper.GetString("listen_addr"))
		mongoUri := config.Viper.GetString("mongo_uri")
		mongoDb := config.Viper.GetString("mongo_db")
		analyticsAddr := config.Viper.GetString("analytics_addr")
		baseFilReward := config.Viper.GetInt64("base_atto_fil_reward")

		if logFile != "" {
			err = cmd.SetupDefaultLoggingConfig(logFile)
			cmd.ErrCheck(err)
		}

		target, err := util.TCPAddrFromMultiAddr(listenAddr)
		cmd.ErrCheck(err)

		listener, err := net.Listen("tcp", target)
		cmd.ErrCheck(err)

		ctx, cancel := context.WithCancel(context.Background())
		conf := service.Config{
			Listener:          listener,
			MongoUri:          mongoUri,
			MongoDbName:       mongoDb,
			AnalyticsAddr:     analyticsAddr,
			BaseAttoFILReward: baseFilReward,
			Debug:             debug,
		}
		api, err := service.New(ctx, conf)
		cmd.ErrCheck(err)

		fmt.Println("Welcome to Hub Filrewards!")

		cmd.HandleInterrupt(func() {
			api.Close()
			cancel()
		})
	},
}
