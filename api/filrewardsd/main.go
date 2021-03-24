package main

import (
	"encoding/json"
	"fmt"
	"net"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/filrewardsd/service"
	"github.com/textileio/textile/v2/cmd"
	"google.golang.org/grpc"
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
				DefValue: "127.0.0.1:5000",
			},
			"mongoUri": {
				Key:      "mongo_uri",
				DefValue: "mongodb://127.0.0.1:27017",
			},
			"mongoDb": {
				Key:      "mongo_db",
				DefValue: "textile_filrewards",
			},
			"mongoAccountsDb": {
				Key:      "mongo_accounts_db",
				DefValue: "textile",
			},
			"analyticsAddr": {
				Key:      "analytics_addr",
				DefValue: "",
			},
			"sendfilAddr": {
				Key:      "sendfil_addr",
				DefValue: "",
			},
			"powAddr": {
				Key:      "pow_addr",
				DefValue: "",
			},
			"isDevnet": {
				Key:      "is_devnet",
				DefValue: false,
			},
			"baseNanoFilReward": {
				Key:      "base_nano_fil_reward",
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
		"mongoAccountsDb",
		config.Flags["mongoAccountsDb"].DefValue.(string),
		"MongoDB accounts database name")

	rootCmd.PersistentFlags().String(
		"analyticsAddr",
		config.Flags["analyticsAddr"].DefValue.(string),
		"Analytics API address")
	rootCmd.PersistentFlags().String(
		"sendfilAddr",
		config.Flags["sendfilAddr"].DefValue.(string),
		"Sendfil API address")
	rootCmd.PersistentFlags().String(
		"powAddr",
		config.Flags["powAddr"].DefValue.(string),
		"Powergate API address")

	rootCmd.PersistentFlags().Bool(
		"isDevnet",
		config.Flags["isDevnet"].DefValue.(bool),
		"Whether or not we're running lotus-devnet")

	rootCmd.PersistentFlags().Int64(
		"baseNanoFilReward",
		config.Flags["baseNanoFilReward"].DefValue.(int64),
		"Base NanoFIL reward amount",
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
		listenAddr := config.Viper.GetString("listen_addr")
		mongoUri := config.Viper.GetString("mongo_uri")
		mongoDb := config.Viper.GetString("mongo_db")
		mongoAccountsDb := config.Viper.GetString("mongo_accounts_db")
		analyticsAddr := config.Viper.GetString("analytics_addr")
		sendfilAddr := config.Viper.GetString("sendfil_addr")
		powAddr := config.Viper.GetString("pow_addr")
		isDevnet := config.Viper.GetBool("is_devnet")
		baseFilReward := config.Viper.GetInt64("base_nano_fil_reward")

		if logFile != "" {
			err = cmd.SetupDefaultLoggingConfig(logFile)
			cmd.ErrCheck(err)
		}

		listener, err := net.Listen("tcp", listenAddr)
		cmd.ErrCheck(err)

		sendfilConn, err := grpc.Dial(sendfilAddr, grpc.WithInsecure())
		cmd.ErrCheck(err)

		conf := service.Config{
			Listener:              listener,
			MongoUri:              mongoUri,
			MongoFilRewardsDbName: mongoDb,
			MongoAccountsDbName:   mongoAccountsDb,
			SendfilClientConn:     sendfilConn,
			AnalyticsAddr:         analyticsAddr,
			PowAddr:               powAddr,
			IsDevnet:              isDevnet,
			BaseNanoFILReward:     baseFilReward,
			Debug:                 debug,
		}
		api, err := service.New(conf)
		cmd.ErrCheck(err)

		fmt.Println("Welcome to Hub Filrewards!")

		cmd.HandleInterrupt(func() {
			cmd.ErrCheck(api.Close())
		})
	},
}
