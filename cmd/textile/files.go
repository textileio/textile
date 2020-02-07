package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	pbar "github.com/cheggaaa/pb/v3"
	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(filesCmd)
	filesCmd.AddCommand(addFileCmd, catFileCmd, lsFilesCmd, rmFileCmd)
}

var filesCmd = &cobra.Command{
	Use: "files",
	Aliases: []string{
		"file",
	},
	Short: "Manage project files",
	Long:  `Manage your project's stored files.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		lsFiles(args)
	},
}

var addFileCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a file",
	Long:  `Add a file to a project folder by path.`,
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

var catFileCmd = &cobra.Command{
	Use:   "cat",
	Short: "Cat a file",
	Long:  `Cat a file from a project folder by path.`,
	Args:  cobra.ExactArgs(2),
	Run: func(c *cobra.Command, args []string) {
		projectID := configViper.GetString("id")
		if projectID == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		info, err := client.GetFile(ctx, args[0], api.Auth{
			Token: authViper.GetString("token"),
		})
		if err != nil {
			cmd.Fatal(err)
		}

		file, err := os.Create(args[1])
		if err != nil {
			cmd.Fatal(err)
		}
		defer file.Close()

		bar := pbar.New(int(info.Size))
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

		ctx2, cancel2 := context.WithTimeout(context.Background(), getFileTimeout)
		defer cancel2()
		if err = client.CatFile(
			ctx2,
			args[0],
			file,
			api.Auth{
				Token: authViper.GetString("token"),
			},
			api.CatWithProgress(progress)); err != nil {
			cmd.Fatal(err)
		}
		bar.SetCurrent(info.Size)
		bar.Finish()

		cmd.Success("Wrote file to: %s", aurora.White(args[1]).Bold())
	},
}

var lsFilesCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List files",
	Long:  `List files in a project folder.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		lsFiles(args)
	},
}

func lsFiles(args []string) {
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

	if len(folder.Entries) > 0 {
		data := make([][]string, len(folder.Entries))
		for i, e := range folder.Entries {
			data[i] = []string{
				e.Path, strconv.Itoa(int(e.Size)), strconv.FormatBool(e.IsDir),
			}
		}
		cmd.RenderTable([]string{"path", "size", "dir"}, data)
	}

	cmd.Message("Found %d files", aurora.White(len(folder.Entries)).Bold())
}

var rmFileCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Remove a file",
	Long:  `Remove a file from a project folder by path.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.RemoveFile(
			ctx,
			args[0],
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Removed file %s", aurora.White(args[0]).Bold())
	},
}
