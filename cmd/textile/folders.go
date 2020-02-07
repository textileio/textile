package main

import (
	"context"
	"errors"
	"strconv"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(foldersCmd)
	filesCmd.AddCommand(addFolderCmd, getFolderCmd, lsFolderEntriesCmd, lsFoldersCmd, rmFolderCmd)

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
		//lsFolders()
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

var getFolderCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a folder",
	Long:  `Get a project folder.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
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
			[]string{"name", "id", "path", "public", "entries", "created"},
			[][]string{{folder.Name, folder.ID, folder.Path, strconv.FormatBool(folder.Public),
				strconv.Itoa(len(folder.Entries)), strconv.Itoa(int(folder.Created))}})
	},
}

var lsFolderEntriesCmd = &cobra.Command{
	Use:   "entries",
	Short: "List folder entries",
	Long:  `List entries in a project folder.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
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

		rows := make([][]string, len(folder.Entries))
		for i, e := range folder.Entries {
			rows[i] = []string{
				e.Path, strconv.Itoa(int(e.Size)), strconv.FormatBool(e.IsDir),
			}
		}

		cmd.RenderTable([]string{"path", "size", "dir"}, rows)
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
		//lsFiles()
	},
}

//func lsFolders() {
//	projectID := configViper.GetString("id")
//	if projectID == "" {
//		cmd.Fatal(errors.New("not a project directory"))
//	}
//
//	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
//	defer cancel()
//	files, err := client.ListFiles(
//		ctx,
//		configViper.GetString("id"),
//		api.Auth{
//			Token: authViper.GetString("token"),
//		})
//	if err != nil {
//		cmd.Fatal(err)
//	}
//
//	if len(files.List) > 0 {
//		data := make([][]string, len(files.List))
//		for i, f := range files.List {
//			data[i] = []string{f.Name, f.Path}
//		}
//		cmd.RenderTable([]string{"name", "path"}, data)
//	}
//
//	cmd.Message("Found %d files", aurora.White(len(files.List)).Bold())
//}

var rmFolderCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Remove a folder",
	Long:  `Remove a project folder (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		//projectID := configViper.GetString("id")
		//if projectID == "" {
		//	cmd.Fatal(errors.New("not a project directory"))
		//}
		//
		//selected := selectFile("Remove file", aurora.Sprintf(
		//	aurora.BrightBlack("> Removing file {{ .Name | white | bold }}")),
		//	projectID)
		//pid, err := cid.Parse(selected.Path)
		//if err != nil {
		//	cmd.Fatal(err)
		//}
		//
		//ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		//defer cancel()
		//if err := client.RemoveFile(
		//	ctx,
		//	path.IpfsPath(pid),
		//	api.Auth{
		//		Token: authViper.GetString("token"),
		//	}); err != nil {
		//	cmd.Fatal(err)
		//}
		//
		//cmd.Success("Removed file %s", aurora.White(selected.Name).Bold())
	},
}

//func selectFolder(label, successMsg, projID string) *pb.GetFileReply {
//	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
//	defer cancel()
//	files, err := client.ListFiles(
//		ctx,
//		projID,
//		api.Auth{
//			Token: authViper.GetString("token"),
//		})
//	if err != nil {
//		cmd.Fatal(err)
//	}
//
//	if len(files.List) == 0 {
//		cmd.End("You don't have any files!")
//	}
//
//	prompt := promptui.Select{
//		Label: label,
//		Items: files.List,
//		Templates: &promptui.SelectTemplates{
//			Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .Name | bold }}`, promptui.IconSelect),
//			Inactive: `{{ .Name | faint }}`,
//			Details:  `{{ "(Path:" | faint }} {{ .Path | faint }}{{ ")" | faint }}`,
//			Selected: successMsg,
//		},
//	}
//	index, _, err := prompt.Run()
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	return files.List[index]
//}
