package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(foldersCmd)
	foldersCmd.AddCommand(
		addFolderCmd,
		lsFoldersCmd,
		inspectFolderCmd,
		rmFolderCmd,
		pushFolderCmd,
		pullFolderCmd)

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
	if configViper.GetString("id") == "" {
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
	Long:  `Display detailed information about a project folder.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		if configViper.GetString("id") == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		folder, err := client.GetFolder(
			ctx,
			args[0],
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
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		if configViper.GetString("id") == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		prompt := promptui.Prompt{
			Label: "Enter the folder name to confirm",
			Validate: func(name string) error {
				if name != args[0] {
					return errors.New("names do not match")
				}
				return nil
			},
		}
		folder, err := prompt.Run()
		if err != nil {
			cmd.End("")
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.RemoveFolder(
			ctx,
			folder,
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Removed folder %s", aurora.White(folder).Bold())
	},
}

var pushFolderCmd = &cobra.Command{
	Use:   "push",
	Short: "Push a folder",
	Long:  `Push a project folder.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		if configViper.GetString("id") == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		var paths []string
		err := filepath.Walk(args[0], func(path string, info os.FileInfo, err error) error {
			if err != nil {
				cmd.Fatal(err)
			}
			if !info.IsDir() {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			cmd.Fatal(err)
		}

		if len(paths) == 0 {
			cmd.End("%s is empty", aurora.White(args[0]).Bold())
		}

		prompt := promptui.Prompt{
			Label: fmt.Sprintf("Add %d files? Press ENTER to confirm", len(paths)),
			Validate: func(in string) error {
				return nil
			},
		}
		_, err = prompt.Run()
		if err != nil {
			cmd.End("")
		}

		for _, p := range paths {
			addFile(p, filepath.Dir(p))
		}
	},
}

var pullFolderCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull a folder",
	Long:  `Pull a project folder.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {

	},
}
