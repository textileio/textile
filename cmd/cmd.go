package cmd

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Flag struct {
	Key      string
	DefValue interface{}
}

func InitConfig(v *viper.Viper, file string, defDir string, name string) func() {
	return func() {
		if file != "" {
			v.SetConfigFile(file)
		} else {
			home, err := homedir.Dir()
			if err != nil {
				panic(err)
			}
			v.AddConfigPath(path.Join("./", defDir)) // local config takes priority
			v.AddConfigPath(path.Join(home, defDir))
			v.SetConfigName(name)
		}

		v.SetEnvPrefix("TXTL")
		v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		v.AutomaticEnv()
		_ = v.ReadInConfig()
	}
}

func BindFlags(v *viper.Viper, root *cobra.Command, flags map[string]Flag) error {
	for n, f := range flags {
		if err := v.BindPFlag(f.Key, root.PersistentFlags().Lookup(n)); err != nil {
			return err
		}
		v.SetDefault(f.Key, f.DefValue)
	}
	return nil
}

func ExpandConfigVars(v *viper.Viper, flags map[string]Flag) {
	for _, f := range flags {
		if f.Key != "" {
			if str, ok := v.Get(f.Key).(string); ok {
				v.Set(f.Key, os.ExpandEnv(str))
			}
		}
	}
}

func AddrFromStr(str string) ma.Multiaddr {
	addr, err := ma.NewMultiaddr(str)
	if err != nil {
		Fatal(err)
	}
	return addr
}

func Fatal(err error) {
	fmt.Println(err)
	os.Exit(1)
}
