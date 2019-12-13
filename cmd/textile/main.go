package main

import (
	logging "github.com/ipfs/go-log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-textile-threads/util"
	"github.com/textileio/textile/cmd"
	logger "github.com/whyrusleeping/go-logging"
)

var (
	log = logging.Logger("textile")

	configFile string
	flags      = map[string]cmd.Flag{
		"debug": {
			Key:      "log.debug",
			DefValue: false,
		},

		"addrApi": {
			Key:      "addr.api",
			DefValue: "/ip4/127.0.0.1/tcp/3006",
		},
	}
)

func init() {
	cobra.OnInitialize(cmd.InitConfig(configFile, ".textile", func() {
		log.Debugf("Using config file: %s", viper.ConfigFileUsed())
	}))

	rootCmd.PersistentFlags().StringVar(
		&configFile,
		"config",
		"",
		"Config file (default ${HOME}/.textiled/config.yaml)")

	rootCmd.PersistentFlags().BoolP(
		"debug",
		"d",
		flags["debug"].DefValue.(bool),
		"Enable debug logging")

	rootCmd.PersistentFlags().String(
		"addrApi",
		flags["addrApi"].DefValue.(string),
		"Textile API listen address")

	if err := cmd.BindFlags(rootCmd, flags); err != nil {
		log.Fatal(err)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		cmd.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   "textile",
	Short: "Textile client",
	Long:  `The Textile client.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(flags)

		if viper.GetBool("log.debug") {
			if err := util.SetLogLevels(map[string]logger.Level{
				"textile": logger.DEBUG,
			}); err != nil {
				log.Fatal(err)
			}
		}
	},
}
