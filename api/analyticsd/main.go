package main

import (
	"encoding/json"
	"fmt"
	"net"

	logging "github.com/ipfs/go-log/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/analyticsd/service"
	"github.com/textileio/textile/v2/cmd"
)

const daemonName = "analyticsd"

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
				DefValue: "/ip4/127.0.0.1/tcp/6006",
			},
			"segmentApiKey": {
				Key:      "segment_api_key",
				DefValue: "",
			},
			"segmentPrefix": {
				Key:      "segment_prefix",
				DefValue: "",
			},
			"filrewardsAddr": {
				Key:      "filrewards_addr",
				DefValue: "",
			},
		},
		EnvPre: "ANALYTICS",
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
		"listenAddr",
		config.Flags["listenAddr"].DefValue.(string),
		"Analytics API listen address")

	// Segment settings
	rootCmd.PersistentFlags().String(
		"segmentApiKey",
		config.Flags["segmentApiKey"].DefValue.(string),
		"Segment API key")
	rootCmd.PersistentFlags().String(
		"segmentPrefix",
		config.Flags["segmentPrefix"].DefValue.(string),
		"Segment trait source prefix")

	rootCmd.PersistentFlags().String(
		"filrewardsAddr",
		config.Flags["filrewardsAddr"].DefValue.(string),
		"Filrewards API address")

	err := cmd.BindFlags(config.Viper, rootCmd, config.Flags)
	cmd.ErrCheck(err)
}

func main() {
	cmd.ErrCheck(rootCmd.Execute())
}

var rootCmd = &cobra.Command{
	Use:   daemonName,
	Short: "Analytics daemon",
	Long:  `Textile's analytics daemon.`,
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

		listenAddr := cmd.AddrFromStr(config.Viper.GetString("listen_addr"))
		segmentApiKey := config.Viper.GetString("segment_api_key")
		segmentPrefix := config.Viper.GetString("segment_prefix")
		filrewardsAddr := config.Viper.GetString("filrewards_addr")
		debug := config.Viper.GetBool("debug")
		logFile := config.Viper.GetString("log_file")

		if logFile != "" {
			err = cmd.SetupDefaultLoggingConfig(logFile)
			cmd.ErrCheck(err)
		}

		target, err := util.TCPAddrFromMultiAddr(listenAddr)
		cmd.ErrCheck(err)

		listener, err := net.Listen("tcp", target)
		cmd.ErrCheck(err)

		api, err := service.New(listener, segmentApiKey, segmentPrefix, filrewardsAddr, debug)
		cmd.ErrCheck(err)

		fmt.Println("Welcome to Hub Analytics!")

		cmd.HandleInterrupt(func() {
			api.Close()
		})
	},
}
