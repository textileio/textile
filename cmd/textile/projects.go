package main

import (
	"context"
	"fmt"
	"os"
	"path"
	"strconv"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/api/pb"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(initCmd, lsCmd, inspectCmd, rmCmd)

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
	Short: "Init a new project",
	Long:  `Initialize a new project.`,
	Run: func(c *cobra.Command, args []string) {
		var pth, name string
		var err error
		if !c.Flag("path").Changed {
			pth, err = os.Getwd()
			if err != nil {
				cmd.Fatal(err)
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
			cmd.Fatal(err)
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
			cmd.Fatal(err)
		}
		configViper.Set("id", proj.ID)
		if proj.StoreID != "" {
			configViper.Set("store", proj.StoreID)
		}

		if err := configViper.WriteConfigAs(filename); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Initialized empty project in %s", aurora.White(pth).Bold())
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
			cmd.Fatal(err)
		}

		if len(projects.List) > 0 {
			data := make([][]string, len(projects.List))
			for i, p := range projects.List {
				data[i] = []string{p.Name, p.ID, p.StoreID}
			}
			cmd.RenderTable([]string{"name", "id", "store id"}, data)
		}

		cmd.Message("Found %d projects for current scope", aurora.White(len(projects.List)).Bold())
	},
}

// @todo: Display something more meaningful when the info is available, e.g., Filecoin and sync info.
var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Display project information",
	Long:  `Display detailed information about a project (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectProject("Select project", aurora.Sprintf(
			aurora.BrightBlack("> Selected {{ .Name | white | bold }}")))

		cmd.RenderTable([]string{"name", "id", "store id", "filecoin wallet address", "filecoin wallet balance"},
			[][]string{{selected.Name, selected.ID, selected.StoreID, selected.WalletAddress, strconv.FormatInt(selected.WalletBalance, 10)}})
	},
}

var rmCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Remove a project",
	Long:  `Remove a project (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectProject("Remove project", aurora.Sprintf(
			aurora.BrightBlack("> Removing project {{ .Name | white | bold }}")))

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.RemoveProject(
			ctx,
			selected.ID,
			api.Auth{
				Token: authViper.GetString("token"),
				Scope: configViper.GetString("scope"),
			}); err != nil {
			cmd.Fatal(err)
		}

		_ = os.RemoveAll(configViper.ConfigFileUsed())

		cmd.Success("Removed project %s", aurora.White(selected.Name).Bold())
	},
}

func selectProject(label, successMsg string) *pb.GetProjectReply {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	projects, err := client.ListProjects(
		ctx,
		api.Auth{
			Token: authViper.GetString("token"),
			Scope: configViper.GetString("scope"),
		})
	if err != nil {
		cmd.Fatal(err)
	}
	if len(projects.List) == 0 {
		cmd.End("You don't have any projects!")
	}

	prompt := promptui.Select{
		Label: label,
		Items: projects.List,
		Templates: &promptui.SelectTemplates{
			Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .Name | bold }}`, promptui.IconSelect),
			Inactive: `{{ .Name | faint }}`,
			Details:  `{{ "(ID:" | faint }} {{ .ID | faint }}{{ ")" | faint }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		log.Fatal(err)
	}

	return projects.List[index]
}
