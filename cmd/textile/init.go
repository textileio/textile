package main

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().String(
		"name",
		"",
		"Project name")

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
		var pth, name string
		var err error
		if !c.Flag("path").Changed {
			pth, err = os.Getwd()
			if err != nil {
				log.Fatal(err)
			}
		} else {
			pth = c.Flag("path").Value.String()
		}
		if !c.Flag("name").Changed {
			name = path.Base(pth)
		} else {
			name = c.Flag("name").Value.String()
		}

		pth = path.Join(pth, ".textile")
		if err := os.MkdirAll(pth, os.ModePerm); err != nil {
			log.Fatal(err)
		}
		filename := path.Join(pth, "config.yml")

		if _, err := os.Stat(filename); err == nil {
			cmd.Fatal(fmt.Errorf("project is already initialized"))
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		proj, err := client.AddProject(
			ctx,
			name,
			authViper.GetString("token"),
			configViper.GetString("scope"))
		if err != nil {
			log.Fatal(err)
		}
		configViper.Set("id", proj.ID)
		if proj.StoreID != "" {
			configViper.Set("store", proj.StoreID)
		}

		if err := configViper.WriteConfigAs(filename); err != nil {
			cmd.Fatal(err)
		}

		fmt.Println(fmt.Sprintf("> Initialized empty project in %s", pth))
	},
}
