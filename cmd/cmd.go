package cmd

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Flag struct {
	Key      string
	DefValue interface{}
}

func InitConfig(file string, defDir string, foundConfig func()) func() {
	return func() {
		if file != "" {
			viper.SetConfigFile(file)
		} else {
			home, err := homedir.Dir()
			if err != nil {
				panic(err)
			}
			viper.AddConfigPath(path.Join(home, defDir))
			viper.SetConfigName("config")
		}

		viper.SetEnvPrefix("TXTL")
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		viper.AutomaticEnv()

		if err := viper.ReadInConfig(); err == nil {
			foundConfig()
		}
	}
}

func BindFlags(root *cobra.Command, flags map[string]Flag) error {
	for n, f := range flags {
		if err := viper.BindPFlag(f.Key, root.PersistentFlags().Lookup(n)); err != nil {
			return err
		}
		viper.SetDefault(f.Key, f.DefValue)
	}
	return nil
}

func ExpandConfigVars(flags map[string]Flag) {
	for _, f := range flags {
		if f.Key != "" {
			if str, ok := viper.Get(f.Key).(string); ok {
				viper.Set(f.Key, os.ExpandEnv(str))
			}
		}
	}
}

func Fatal(err error) {
	fmt.Println(err)
	os.Exit(1)
}
