package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(createCmd)

	createCmd.Flags().String(
		"dir",
		"",
		"Directory to write config (default ${HOME}/.textiled)")
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Config utils",
	Long:  `Config file utilities.`,
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create config",
	Long:  `Create a config file.`,
	Run: func(c *cobra.Command, args []string) {
		var dir string
		if !c.Flag("dir").Changed {
			home, err := homedir.Dir()
			if err != nil {
				log.Fatal(err)
			}
			dir = filepath.Join(home, ".textiled")
			if err = os.MkdirAll(dir, os.ModePerm); err != nil {
				log.Fatal(err)
			}
		} else {
			dir = c.Flag("dir").Value.String()
		}
		filename := filepath.Join(dir, "config.yml")

		if _, err := os.Stat(filename); err == nil {
			cmd.Fatal(fmt.Errorf("%s already exists", filename))
		}

		if err := configViper.WriteConfigAs(filename); err != nil {
			cmd.Fatal(err)
		}
	},
}
