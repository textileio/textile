package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	pbar "github.com/cheggaaa/pb/v3"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(filesCmd)
	filesCmd.AddCommand(addFileCmd, lsFilesCmd, rmFileCmd)
}

var filesCmd = &cobra.Command{
	Use: "files",
	Aliases: []string{
		"file",
	},
	Short: "Manage project files",
	Long:  `Manage your project's stored files.`,
	Run: func(c *cobra.Command, args []string) {
		//lsFiles()
	},
}

var addFileCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a file",
	Long:  `Add a file to a project folder.`,
	Args:  cobra.ExactArgs(2),
	Run: func(c *cobra.Command, args []string) {
		projectID := configViper.GetString("id")
		if projectID == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		file, err := os.Open(args[0])
		if err != nil {
			cmd.Fatal(err)
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			cmd.Fatal(err)
		}
		filePath := filepath.Join(args[1], filepath.Base(info.Name()))

		bar := pbar.New(int(info.Size()))
		bar.SetTemplate(pbar.Full)
		bar.Set(pbar.Bytes, true)
		bar.Set(pbar.SIBytesPrefix, true)
		bar.Start()
		progress := make(chan int64)
		go func() {
			for up := range progress {
				bar.SetCurrent(up)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), addFileTimeout)
		defer cancel()
		pth, err := client.AddFile(
			ctx,
			filePath,
			file,
			api.Auth{
				Token: authViper.GetString("token"),
			},
			api.AddWithProgress(progress))
		if err != nil {
			cmd.Fatal(err)
		}
		bar.Finish()

		cmd.Success("Added file at path: %s", aurora.White(pth.String()).Bold())
	},
}

var lsFilesCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List files",
	Long:  `List files in a project folder.`,
	Run: func(c *cobra.Command, args []string) {
		//lsFiles()
	},
}

//func lsFiles() {
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

var rmFileCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Remove a file",
	Long:  `Remove a file from a project folder (interactive).`,
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

//func selectFile(label, successMsg, projID string) *pb.GetFileReply {
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
