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
)

// InitConfigCmd adds a config generator command to the root command.
// The command will write the config file to dir.
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

// InitConfig returns a function that can be used to search for and load a config file.
func InitConfig(conf *Config) func() {
	return func() {
		FindConfigFile(conf, ".")
	}
}

// FindConfigFile searches up the path for a config file.
// True is returned is a config file was found and successfully loaded.
func FindConfigFile(conf *Config, pth string) bool {
	found := false
	h := 1
	for h <= maxSearchHeight && !found {
		found = initConfig(conf.Viper, conf.File, pth, conf.Dir, conf.Name, conf.EnvPre, conf.Global)
		npth := filepath.Dir(pth)
		if npth == string(os.PathSeparator) && pth == string(os.PathSeparator) {
			return found
		}
		pth = npth
		h++
	}
	return found
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

// WriteConfig writes the viper config based on the command.
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
		Fatal(fmt.Errorf("%s already exists", filename))
	}
	if err := v.WriteConfigAs(filename); err != nil {
		Fatal(err)
	}
}

// WriteConfigToHome writes config to the home directory.
func WriteConfigToHome(config *Config) {
	home, err := homedir.Dir()
	ErrCheck(err)
	dir := filepath.Join(home, config.Dir)
	err = os.MkdirAll(dir, os.ModePerm)
	ErrCheck(err)
	filename := filepath.Join(dir, config.Name+".yml")
	err = config.Viper.WriteConfigAs(filename)
	ErrCheck(err)
}

// BindFlags binds the flags to the viper config values.
func BindFlags(v *viper.Viper, root *cobra.Command, flags map[string]Flag) error {
	for n, f := range flags {
		if err := v.BindPFlag(f.Key, root.PersistentFlags().Lookup(n)); err != nil {
			return err
		}
		v.SetDefault(f.Key, f.DefValue)
	}
	return nil
}

// GetFlagOrEnvValue first load a value for the key from the command flags.
// If no value was found, the value for the corresponding env variable is returned.
func GetFlagOrEnvValue(c *cobra.Command, k, envPre string) (v string) {
	changed := c.Flags().Changed(k)
	v, err := c.Flags().GetString(k)
	if err == nil && changed {
		return
	}
	env := os.Getenv(fmt.Sprintf("%s_%s", envPre, strings.ToUpper(k)))
	if env != "" {
		return env
	}
	return v
}

// ExpandConfigVars evaluates the viper config file's expressions.
func ExpandConfigVars(v *viper.Viper, flags map[string]Flag) {
	for _, f := range flags {
		if f.Key != "" {
			if str, ok := v.Get(f.Key).(string); ok {
				v.Set(f.Key, os.ExpandEnv(str))
			}
		}
	}
}

// AddrFromStr returns a multiaddress from the string.
func AddrFromStr(str string) ma.Multiaddr {
	addr, err := ma.NewMultiaddr(str)
	if err != nil {
		Fatal(err)
	}
	return addr
}
