package main

import (
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().String(
		"path",
		".",
		"Project path")
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Init",
	Long:  `Initialize a new project.`,
	Run: func(c *cobra.Command, args []string) {
		pth := "."
		if c.Flag("path").Changed {
			pth = c.Flag("path").Value.String()
		}
		pth = path.Join(pth, ".textile")
		if err := os.MkdirAll(pth, os.ModePerm); err != nil {
			log.Fatal(err)
		}
		filename := path.Join(pth, "config.yml")

		if _, err := os.Stat(filename); err == nil {
			cmd.Fatal(fmt.Errorf("project is already initialized"))
		}

		if err := configViper.WriteConfigAs(filename); err != nil {
			cmd.Fatal(err)
		}
	},
}
