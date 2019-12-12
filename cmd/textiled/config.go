package main

import (
	"fmt"
	"os"
	"path"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
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
	Long:  `Create a default config file.`,
	Run: func(cmd *cobra.Command, args []string) {
		viper.SetConfigType("yaml")

		var dir string
		if !cmd.Flag("dir").Changed {
			home, err := homedir.Dir()
			if err != nil {
				log.Fatal(err)
			}
			dir = path.Join(home, ".textiled")
			_ = os.MkdirAll(dir, os.ModePerm)
		} else {
			dir = cmd.Flag("dir").Value.String()
		}
		filename := path.Join(dir, "config.yaml")

		if _, err := os.Stat(filename); err == nil {
			fmt.Println(fmt.Sprintf("%s already exists", filename))
			os.Exit(1)
		}

		if err := viper.WriteConfigAs(filename); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}
