package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/powergate/exe/cli/cmd"
)

func InitConfigCmd(rootCmd *cobra.Command, v *viper.Viper, dir string) {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Config utils",
		Long:  `Config file utilities.`,
	}
	rootCmd.AddCommand(configCmd)
	createCmd := &cobra.Command{
		Use:   "create",
		Short: "Create config",
		Long:  `Create a config file.`,
		Run: func(c *cobra.Command, args []string) {
			WriteConfig(c, v, dir)
		},
	}
	configCmd.AddCommand(createCmd)
	createCmd.Flags().String(
		"dir",
		"",
		"Directory to write config (default ${HOME}/"+dir+")")
}

const maxSearchHeight = 50

func InitConfig(conf Config) func() {
	return func() {
		found := false
		pre := "."
		h := 1
		for h <= maxSearchHeight && !found {
			found = initConfig(conf.Viper, conf.File, pre, conf.Dir, conf.Name, conf.EnvPre, conf.Global)
			pre = filepath.Join("../", pre)
			h++
		}
	}
}

func initConfig(v *viper.Viper, file, pre, cdir, name, envPre string, global bool) bool {
	if file != "" {
		v.SetConfigFile(file)
	} else {
		v.AddConfigPath(filepath.Join(pre, cdir)) // local config takes priority
		if global {
			home, err := homedir.Dir()
			if err != nil {
				panic(err)
			}
			v.AddConfigPath(filepath.Join(home, cdir))
		}
		v.SetConfigName(name)
	}

	v.SetEnvPrefix(envPre)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil && strings.Contains(err.Error(), "Not Found") {
		return false
	}
	return true
}

func WriteConfig(c *cobra.Command, v *viper.Viper, name string) {
	var dir string
	if !c.Flag("dir").Changed {
		home, err := homedir.Dir()
		if err != nil {
			Fatal(err)
		}
		dir = filepath.Join(home, name)
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			Fatal(err)
		}
	} else {
		dir = c.Flag("dir").Value.String()
	}

	filename := filepath.Join(dir, "config.yml")
	if _, err := os.Stat(filename); err == nil {
		cmd.Fatal(fmt.Errorf("%s already exists", filename))
	}
	if err := v.WriteConfigAs(filename); err != nil {
		cmd.Fatal(err)
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

func ThreadIDFromString(str string) thread.ID {
	if str == "" {
		return ""
	}
	id, err := thread.Decode(str)
	if err != nil {
		cmd.Fatal(err)
	}
	return id
}
