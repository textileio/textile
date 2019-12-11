package cmd

import (
	"fmt"
	"os"
	"strconv"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "textile",
	Short: "Textile client and daemon",
	Long:  `The Textile client and daemon.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		viper.AddConfigPath(home)
		viper.SetConfigName(".textile")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func getStringFlag(f *pflag.Flag) string {
	if f == nil {
		return ""
	}
	if f.Changed {
		return f.Value.String()
	} else {
		return f.DefValue
	}
}

func getBoolFlag(f *pflag.Flag) bool {
	if f == nil {
		return false
	}
	var str string
	if f.Changed {
		str = f.Value.String()
	} else {
		str = f.DefValue
	}
	val, err := strconv.ParseBool(str)
	if err != nil {
		return false
	}
	return val
}
