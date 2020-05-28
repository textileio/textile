package cmd

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func InitConfigCmd(root *cobra.Command, v *viper.Viper, dir string) {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Config utils",
		Long:  `Config file utilities.`,
	}
	root.AddCommand(configCmd)
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
