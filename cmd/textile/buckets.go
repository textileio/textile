package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/textileio/textile/api/pb"

	pbar "github.com/cheggaaa/pb/v3"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(bucketsCmd)
	bucketsCmd.AddCommand(
		lsBucketPathCmd,
		pushBucketPathCmd,
		pullBucketPathCmd,
		rmBucketPathCmd)
}

var bucketsCmd = &cobra.Command{
	Use: "buckets",
	Aliases: []string{
		"bucket",
	},
	Short: "Manage project buckets",
	Long:  `Manage your project's buckets.`,
	Run: func(c *cobra.Command, args []string) {
		lsBucketPath(args)
	},
}

var lsBucketPathCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List bucket path contents",
	Long:  `List files and directories under a bucket path.`,
	Run: func(c *cobra.Command, args []string) {
		lsBucketPath(args)
	},
}

func lsBucketPath(args []string) {
	if configViper.GetString("id") == "" {
		cmd.Fatal(errors.New("not a project directory"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	token := authViper.GetString("token")
	var data [][]string
	var count int
	if len(args) == 0 {
		buckets, err := client.ListBuckets(ctx, configViper.GetString("id"), api.Auth{Token: token})
		if err != nil {
			cmd.Fatal(err)
		}
		if len(buckets.List) > 0 {
			data = make([][]string, len(buckets.List))
			for i, r := range buckets.List {
				data[i] = []string{
					r.Root.Name,
					strconv.Itoa(int(r.Item.Size)),
					strconv.FormatBool(r.Item.IsDir),
					strconv.Itoa(len(r.Item.Items)),
					r.Item.Path,
				}
			}
			count = len(buckets.List)
		}
	} else {
		rep, err := client.GetBucketPath(ctx, args[0], api.Auth{Token: token})
		if err != nil {
			cmd.Fatal(err)
		}
		var items []*pb.GetBucketPathReply_Item
		if len(rep.Item.Items) > 0 {
			items = rep.Item.Items
		} else if !rep.Item.IsDir {
			items = append(items, rep.Item)
		}
		if len(items) > 0 {
			data = make([][]string, len(items))
			for i, item := range items {
				data[i] = []string{
					filepath.Base(item.Path),
					strconv.Itoa(int(item.Size)),
					strconv.FormatBool(item.IsDir),
					strconv.Itoa(len(item.Items)),
					item.Path,
				}
			}
			count = len(items)
		}
	}
	if len(data) > 0 {
		cmd.RenderTable([]string{"name", "size", "dir", "items", "path"}, data)
	}

	cmd.Message("Found %d items", aurora.White(count).Bold())
}

var pushBucketPathCmd = &cobra.Command{
	Use:   "push",
	Short: "Push to a bucket path",
	Long: `Push files and directories to a bucket path. All paths will be created if they don't exist.

File structure is mirrored in the bucket. For example, given the directory:
    foo/one.txt
    foo/bar/two.txt
    foo/bar/baz/three.txt

These 'push' commands result in the following bucket structures.

'textile buckets push foo mybuck':
    mybuck/foo/one.txt
    mybuck/foo/bar/two.txt
    mybuck/foo/bar/baz/three.txt

'textile buckets push foo/bar mybuck':
    mybuck/bar/two.txt
    mybuck/bar/baz/three.txt

'textile buckets push foo/bar/baz mybuck':
    mybuck/baz/three.txt

'textile buckets push foo/bar/baz/three.txt mybuck':
    mybuck/three.txt

'textile buckets push foo/* mybuck':
    mybuck/one.txt
    mybuck/bar/two.txt
    mybuck/bar/baz/three.txt
`,
	Args: cobra.MinimumNArgs(2),
	Run: func(c *cobra.Command, args []string) {
		projectID := configViper.GetString("id")
		if projectID == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		var names []string
		var paths []string
		bucketPath, args := args[len(args)-1], args[:len(args)-1]
		for _, a := range args {
			dir, _ := filepath.Split(a)
			err := filepath.Walk(a, func(n string, info os.FileInfo, err error) error {
				if err != nil {
					cmd.Fatal(err)
				}
				if !info.IsDir() {
					names = append(names, n)
					var p string
					if n == a { // This is a file given as an arg
						// In this case, the bucket path should not include the directory
						p = filepath.Join(bucketPath, info.Name())
					} else { // This is a directory given as an arg, or one of its sub directories
						// The bucket path should maintain directory structure
						p = filepath.Join(bucketPath, strings.TrimPrefix(n, dir))
					}
					paths = append(paths, p)
				}
				return nil
			})
			if err != nil {
				cmd.Fatal(err)
			}
		}
		if len(names) == 0 {
			cmd.End("No files found")
		}

		prompt := promptui.Prompt{
			Label: fmt.Sprintf("Add %d files? Press ENTER to confirm", len(names)),
			Validate: func(in string) error {
				return nil
			},
		}
		if _, err := prompt.Run(); err != nil {
			cmd.End("")
		}

		for i := range names {
			addFile(projectID, names[i], paths[i])
		}

		cmd.Success("Pushed %d files to %s", len(names), aurora.White(bucketPath).Bold())
	},
}

func addFile(projectID, name, filePath string) (path.Resolved, path.Path) {
	file, err := os.Open(name)
	if err != nil {
		cmd.Fatal(err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		cmd.Fatal(err)
	}

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
	pth, root, err := client.PushBucketPath(
		ctx,
		projectID,
		filePath,
		file,
		api.Auth{
			Token: authViper.GetString("token"),
		},
		api.WithPushProgress(progress))
	if err != nil {
		cmd.Fatal(err)
	}
	bar.Finish()

	cmd.Message("Pushed %s to %s", aurora.White(name).Bold(), aurora.White(filePath).Bold())

	return pth, root
}

var pullBucketPathCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull a bucket path",
	Long:  `Pull files and directories from a bucket path.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {

	},
}

var rmBucketPathCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Remove bucket path contents",
	Long:  `Remove files and directories under a bucket path.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		if configViper.GetString("id") == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.RemoveBucketPath(
			ctx,
			args[0],
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Removed %s", aurora.White(args[0]).Bold())
	},
}

//var catFileCmd = &cobra.Command{
//	Use:   "cat",
//	Short: "Cat a file",
//	Long:  `Cat a file from a project folder by path.`,
//	Args:  cobra.ExactArgs(2),
//	Run: func(c *cobra.Command, args []string) {
//		if configViper.GetString("id") == "" {
//			cmd.Fatal(errors.New("not a project directory"))
//		}
//
//		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
//		defer cancel()
//		info, err := client.GetFile(ctx, args[0], api.Auth{
//			Token: authViper.GetString("token"),
//		})
//		if err != nil {
//			cmd.Fatal(err)
//		}
//
//		file, err := os.Create(args[1])
//		if err != nil {
//			cmd.Fatal(err)
//		}
//		defer file.Close()
//
//		bar := pbar.New(int(info.Size))
//		bar.SetTemplate(pbar.Full)
//		bar.Set(pbar.Bytes, true)
//		bar.Set(pbar.SIBytesPrefix, true)
//		bar.Start()
//		progress := make(chan int64)
//		go func() {
//			for up := range progress {
//				bar.SetCurrent(up)
//			}
//		}()
//
//		ctx2, cancel2 := context.WithTimeout(context.Background(), getFileTimeout)
//		defer cancel2()
//		if err = client.CatFile(
//			ctx2,
//			args[0],
//			file,
//			api.Auth{
//				Token: authViper.GetString("token"),
//			},
//			api.CatWithProgress(progress)); err != nil {
//			cmd.Fatal(err)
//		}
//		bar.SetCurrent(info.Size)
//		bar.Finish()
//
//		cmd.Success("Wrote file to %s", aurora.White(args[1]).Bold())
//	},
//}
