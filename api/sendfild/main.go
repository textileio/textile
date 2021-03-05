package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/powergate/v2/lotus"
	"github.com/textileio/textile/v2/api/sendfild/service"
	"github.com/textileio/textile/v2/cmd"
)

const daemonName = "sendfild"

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
			"lotusAddr": {
				Key:      "lotus_addr",
				DefValue: "127.0.0.1:7777",
			},
			"lotusAuthToken": {
				Key:      "lotus_auth_token",
				DefValue: "",
			},
			"lotusConnRetries": {
				Key:      "lotus_conn_retries",
				DefValue: 2,
			},
			"mongoUri": {
				Key:      "mongo_uri",
				DefValue: "mongodb://127.0.0.1:27017",
			},
			"mongoDb": {
				Key:      "mongo_db",
				DefValue: "textile_sendfil",
			},
			"messageWaitTimeout": {
				Key:      "message_wait_timeout",
				DefValue: time.Minute * 5,
			},
			"messageConfidence": {
				Key:      "message_confidence",
				DefValue: uint64(5),
			},
			"retryWaitFrequency": {
				Key:      "retry_wait_frequency",
				DefValue: time.Minute,
			},
		},
		EnvPre: "SENDFIL",
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
		"Sendfil API listen address")

	rootCmd.PersistentFlags().String(
		"lotusAddr",
		config.Flags["lotusAddr"].DefValue.(string),
		"Lotus API address")
	rootCmd.PersistentFlags().String(
		"lotusAuthToken",
		config.Flags["lotusAuthToken"].DefValue.(string),
		"Lotus API auth token")
	rootCmd.PersistentFlags().Int(
		"lotusConnRetries",
		config.Flags["lotusConnRetries"].DefValue.(int),
		"Lotus API connection retry count")

	rootCmd.PersistentFlags().String(
		"mongoUri",
		config.Flags["mongoUri"].DefValue.(string),
		"MongoDB connection URI")
	rootCmd.PersistentFlags().String(
		"mongoDb",
		config.Flags["mongoDb"].DefValue.(string),
		"MongoDB database name")

	rootCmd.PersistentFlags().Duration(
		"messageWaitTimeout",
		config.Flags["messageWaitTimeout"].DefValue.(time.Duration),
		"Timeout for listening for messages to become active on chain")
	rootCmd.PersistentFlags().Uint64(
		"messageConfidence",
		config.Flags["messageConfidence"].DefValue.(uint64),
		"Confidence, in epochs, used to consider a message active on chain")
	rootCmd.PersistentFlags().Duration(
		"retryWaitFrequency",
		config.Flags["retryWaitFrequency"].DefValue.(time.Duration),
		"Frequency with which to query for txns that need to be monitored for completion")

	err := cmd.BindFlags(config.Viper, rootCmd, config.Flags)
	cmd.ErrCheck(err)
}

func main() {
	cmd.ErrCheck(rootCmd.Execute())
}

var rootCmd = &cobra.Command{
	Use:   daemonName,
	Short: "Sendfil daemon",
	Long:  `Textile's sendfil daemon.`,
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
		lotusAddr := config.Viper.GetString("lotus_addr")
		lotusAuthToken := config.Viper.GetString("lotus_auth_token")
		lotusConnRetries := config.Viper.GetInt("lotus_conn_retries")
		mongoUri := config.Viper.GetString("mongo_uri")
		mongoDb := config.Viper.GetString("mongo_db")
		messageTimeout := config.Viper.GetDuration("message_timeout")
		messageConfidence := config.Viper.GetUint64("message_confidence")
		retryWaitFrequency := config.Viper.GetDuration("retry_wait_frequency")

		if logFile != "" {
			err = cmd.SetupDefaultLoggingConfig(logFile)
			cmd.ErrCheck(err)
		}

		listener, err := net.Listen("tcp", listenAddr)
		cmd.ErrCheck(err)

		cb, err := lotus.NewBuilder(lotusAddr, lotusAuthToken, lotusConnRetries)
		cmd.ErrCheck(err)

		conf := service.Config{
			Listener:           listener,
			ClientBuilder:      cb,
			MongoUri:           mongoUri,
			MongoDbName:        mongoDb,
			MessageWaitTimeout: messageTimeout,
			MessageConfidence:  messageConfidence,
			RetryWaitFrequency: retryWaitFrequency,
			Debug:              debug,
		}
		api, err := service.New(conf)
		cmd.ErrCheck(err)

		fmt.Println("Welcome to Hub Sendfil!")

		cmd.HandleInterrupt(func() {
			cmd.ErrCheck(api.Close())
		})
	},
}
