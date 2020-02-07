package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/api/pb"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(foldersCmd)
	foldersCmd.AddCommand(addFolderCmd, lsFoldersCmd, inspectFolderCmd, rmFolderCmd)

	addFolderCmd.Flags().Bool(
		"public",
		false,
		"Make folder public")
}

var foldersCmd = &cobra.Command{
	Use: "folders",
	Aliases: []string{
		"folder",
	},
	Short: "Manage project folders",
	Long:  `Manage your project's stored folders.`,
	Run: func(c *cobra.Command, args []string) {
		lsFolders()
	},
}

var addFolderCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new folder",
	Long:  `Add a new project folder.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		projectID := configViper.GetString("id")
		if projectID == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		public, err := c.Flags().GetBool("public")
		if err != nil {
			cmd.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		folder, err := client.AddFolder(
			ctx,
			projectID,
			args[0],
			public,
			api.Auth{
				Token: authViper.GetString("token"),
			})
		if err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Added folder at path: %s", aurora.White(folder.Path).Bold())
	},
}

var lsFoldersCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List folders",
	Long:  `List project folders.`,
	Run: func(c *cobra.Command, args []string) {
		lsFolders()
	},
}

func lsFolders() {
	projectID := configViper.GetString("id")
	if projectID == "" {
		cmd.Fatal(errors.New("not a project directory"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	folders, err := client.ListFolders(
		ctx,
		configViper.GetString("id"),
		api.Auth{
			Token: authViper.GetString("token"),
		})
	if err != nil {
		cmd.Fatal(err)
	}

	if len(folders.List) > 0 {
		data := make([][]string, len(folders.List))
		for i, f := range folders.List {
			data[i] = []string{f.Name, f.Path}
		}
		cmd.RenderTable([]string{"name", "path"}, data)
	}

	cmd.Message("Found %d folders", aurora.White(len(folders.List)).Bold())
}

var inspectFolderCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Display folder information",
	Long:  `Display detailed information about a project folder (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		projectID := configViper.GetString("id")
		if projectID == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		selected := selectFolder("Select folder", aurora.Sprintf(
			aurora.BrightBlack("> Selected {{ .Name | white | bold }}")),
			projectID)

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		folder, err := client.GetFolder(
			ctx,
			selected.Name,
			api.Auth{
				Token: authViper.GetString("token"),
			})
		if err != nil {
			cmd.Fatal(err)
		}

		cmd.RenderTable(
			[]string{"name", "id", "path", "public", "files", "created"},
			[][]string{{
				folder.Name,
				folder.ID,
				folder.Path,
				strconv.FormatBool(folder.Public),
				strconv.Itoa(len(folder.Entries)),
				strconv.Itoa(int(folder.Created)),
			}})
	},
}

var rmFolderCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Remove a folder",
	Long:  `Remove a project folder (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		projectID := configViper.GetString("id")
		if projectID == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		selected := selectFolder("Remove folder", aurora.Sprintf(
			aurora.BrightBlack("> Removing folder {{ .Name | white | bold }}")),
			projectID)

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.RemoveFolder(
			ctx,
			selected.Name,
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Removed folder %s", aurora.White(selected.Name).Bold())
	},
}

func selectFolder(label, successMsg, projID string) *pb.GetFolderReply {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	folders, err := client.ListFolders(
		ctx,
		projID,
		api.Auth{
			Token: authViper.GetString("token"),
		})
	if err != nil {
		cmd.Fatal(err)
	}

	if len(folders.List) == 0 {
		cmd.End("You don't have any folders!")
	}

	prompt := promptui.Select{
		Label: label,
		Items: folders.List,
		Templates: &promptui.SelectTemplates{
			Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .Name | bold }}`, promptui.IconSelect),
			Inactive: `{{ .Name | faint }}`,
			Details:  `{{ "(Path:" | faint }} {{ .Path | faint }}{{ ")" | faint }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		log.Fatal(err)
	}

	return folders.List[index]
}
