package main

import (
	"context"
	"fmt"
	"os"
	"path"

	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(lsCmd, inspectCmd, rmCmd)

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
			api.Auth{
				Token: authViper.GetString("token"),
				Scope: configViper.GetString("scope"),
			})
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

var lsCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List projects",
	Long:  `List existing projects under the current scope.`,
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		projects, err := client.ListProjects(
			ctx,
			api.Auth{
				Token: authViper.GetString("token"),
				Scope: configViper.GetString("scope"),
			})
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(fmt.Sprintf("> Found %d projects\n", len(projects.List)))

		data := make([][]string, len(projects.List))
		for i, p := range projects.List {
			data[i] = []string{p.Name, p.ID, p.StoreID}
		}
		cmd.RenderTable([]string{"name", "id", "store id"}, data)
	},
}

// @todo: Display something more meaningful when the info is available, e.g., Filecoin and sync info.
var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Display project information",
	Long:  `Display detailed information about a project.`,
	Args: func(c *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("requires an ID argument")
		}
		return nil
	},
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		proj, err := client.GetProject(
			ctx,
			args[0],
			api.Auth{
				Token: authViper.GetString("token"),
				Scope: configViper.GetString("scope"),
			})
		if err != nil {
			log.Fatal(err)
		}

		cmd.RenderTable([]string{"name", "id", "store id"},
			[][]string{{proj.Name, proj.ID, proj.StoreID}})
	},
}

var rmCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Remove a project",
	Long:  `Removes a project by its unique identifier (ID).`,
	Args: func(c *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("requires an ID argument")
		}
		return nil
	},
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.RemoveProject(
			ctx,
			args[0],
			api.Auth{
				Token: authViper.GetString("token"),
				Scope: configViper.GetString("scope"),
			}); err != nil {
			log.Fatal(err)
		}
		fmt.Println("> Success")
	},
}
