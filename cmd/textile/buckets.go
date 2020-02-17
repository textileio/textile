package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	pbar "github.com/cheggaaa/pb/v3"
	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/api/pb"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(bucketsCmd)
	bucketsCmd.AddCommand(
		lsBucketPathCmd,
		pushBucketPathCmd,
		pullBucketPathCmd,
		catBucketPathCmd,
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
	Use: "ls [path]",
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
	project := configViper.GetString("project")
	if project == "" {
		cmd.Fatal(errors.New("not a project directory"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()

	var pth string
	if len(args) > 0 {
		pth = args[0]
	}
	if pth == "." || pth == "/" || pth == "./" {
		pth = ""
	}
	rep, err := client.ListBucketPath(ctx, project, pth, api.Auth{
		Token: authViper.GetString("token"),
	})
	if err != nil {
		cmd.Fatal(err)
	}
	var items []*pb.ListBucketPathReply_Item
	if len(rep.Item.Items) > 0 {
		items = rep.Item.Items
	} else if !rep.Item.IsDir {
		items = append(items, rep.Item)
	}

	var data [][]string
	if len(items) > 0 {
		data = make([][]string, len(items))
		for i, item := range items {
			var links string
			if item.IsDir {
				links = strconv.Itoa(len(item.Items))
			} else {
				links = "n/a"
			}
			data[i] = []string{
				item.Name,
				strconv.Itoa(int(item.Size)),
				strconv.FormatBool(item.IsDir),
				links,
				item.Path,
			}
		}
	}

	if len(data) > 0 {
		cmd.RenderTable([]string{"name", "size", "dir", "items", "path"}, data)
	}

	cmd.Message("Found %d items", aurora.White(len(items)).Bold())
}

var pushBucketPathCmd = &cobra.Command{
	Use:   "push [target] [path]",
	Short: "Push to a bucket path",
	Long: `Push files and directories to a bucket path. Existing paths will be overwritten. Non-existing paths will be created.

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

'textile buckets push foo/* foo':
    foo/one.txt
    foo/bar/two.txt
    foo/bar/baz/three.txt
`,
	Args: cobra.MinimumNArgs(2),
	Run: func(c *cobra.Command, args []string) {
		project := configViper.GetString("project")
		if project == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		var names []string
		var paths []string
		bucketPath, args := args[len(args)-1], args[:len(args)-1]
		for _, a := range args {
			dir := filepath.Dir(a)
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
			addFile(project, names[i], paths[i])
		}

		cmd.Success("Pushed %d files to %s", aurora.White(len(names)).Bold(), aurora.White(bucketPath).Bold())
	},
}

// @todo: Support Stdin
func addFile(project, name, filePath string) {
	file, err := os.Open(name)
	if err != nil {
		cmd.Fatal(err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		cmd.Fatal(err)
	}

	cmd.Message("Pushing %s to %s", aurora.White(name).Bold(), aurora.White(filePath).Bold())

	bar := pbar.New(int(info.Size()))
	bar.SetTemplate(pbar.Full)
	bar.Set(pbar.Bytes, true)
	bar.Set(pbar.SIBytesPrefix, true)
	bar.Start()
	progress := make(chan int64)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), addFileTimeout)
		defer cancel()
		if _, _, err = client.PushBucketPath(
			ctx,
			project,
			filePath,
			file,
			api.Auth{
				Token: authViper.GetString("token"),
			},
			api.WithPushProgress(progress)); err != nil {
			if strings.HasSuffix(err.Error(), "Scope does not own project") {
				bucket := strings.SplitN(filePath, "/", 2)[0]
				msg := aurora.Sprintf(aurora.BrightBlack(
					"a bucket with name %s is already in use, try again (names are global)"),
					aurora.Cyan(bucket))
				cmd.Fatal(fmt.Errorf(msg))
			}
			cmd.Fatal(err)
		}
	}()

	for up := range progress {
		bar.SetCurrent(up)
	}
	bar.Finish()
}

var pullBucketPathCmd = &cobra.Command{
	Use:   "pull [path] [destination]",
	Short: "Pull a bucket path",
	Long: `Pull files and directories from a bucket path. Existing paths will be overwritten. Non-existing paths will be created.

Bucket structure is mirrored locally. For example, given the bucket:
    foo/one.txt
    foo/bar/two.txt
    foo/bar/baz/three.txt

These 'pull' commands result in the following local structures.

'textile buckets pull foo mydir':
    mydir/foo/one.txt
    mydir/foo/bar/two.txt
    mydir/foo/bar/baz/three.txt

'textile buckets pull foo/bar mydir':
    mydir/bar/two.txt
    mydir/bar/baz/three.txt

'textile buckets pull foo/bar/baz mydir':
    mydir/baz/three.txt

'textile buckets pull foo/bar/baz/three.txt mydir':
    mydir/three.txt

'textile buckets pull foo .':
    foo/one.txt
    foo/bar/two.txt
    foo/bar/baz/three.txt
`,
	Args: cobra.ExactArgs(2),
	Run: func(c *cobra.Command, args []string) {
		project := configViper.GetString("project")
		if project == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		count := getPath(project, args[0], filepath.Dir(args[0]), args[1])

		cmd.Success("Pulled %d files to %s", aurora.White(count).Bold(), aurora.White(args[1]).Bold())
	},
}

func getPath(project, pth, dir, dest string) (count int) {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	rep, err := client.ListBucketPath(ctx, project, pth, api.Auth{
		Token: authViper.GetString("token"),
	})
	if err != nil {
		cmd.Fatal(err)
	}

	if rep.Item.IsDir {
		for _, i := range rep.Item.Items {
			count += getPath(project, filepath.Join(pth, filepath.Base(i.Path)), dir, dest)
		}
	} else {
		name := filepath.Join(dest, strings.TrimPrefix(pth, dir))
		getFile(pth, name, rep.Item.Size)
		count++
	}
	return count
}

func getFile(filePath, name string, size int64) {
	if err := os.MkdirAll(filepath.Dir(name), os.ModePerm); err != nil {
		cmd.Fatal(err)
	}
	file, err := os.Create(name)
	if err != nil {
		cmd.Fatal(err)
	}
	defer file.Close()

	cmd.Message("Pulling %s to %s", aurora.White(filePath).Bold(), aurora.White(name).Bold())

	bar := pbar.New(int(size))
	bar.SetTemplate(pbar.Full)
	bar.Set(pbar.Bytes, true)
	bar.Set(pbar.SIBytesPrefix, true)
	bar.Start()
	progress := make(chan int64)

	go func() {
		ctx2, cancel2 := context.WithTimeout(context.Background(), getFileTimeout)
		defer cancel2()
		if err = client.PullBucketPath(
			ctx2,
			filePath,
			file,
			api.Auth{
				Token: authViper.GetString("token"),
			},
			api.WithPullProgress(progress)); err != nil {
			cmd.Fatal(err)
		}
	}()

	for up := range progress {
		bar.SetCurrent(up)
	}
	bar.Finish()
}

var catBucketPathCmd = &cobra.Command{
	Use:   "cat [path]",
	Short: "Cat a bucket path file",
	Long:  `Cat a file at a bucket path.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		project := configViper.GetString("project")
		if project == "" {
			cmd.Fatal(errors.New("not a project directory"))
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.PullBucketPath(ctx, args[0], os.Stdout, api.Auth{
			Token: authViper.GetString("token"),
		}); err != nil {
			cmd.Fatal(err)
		}
	},
}

var rmBucketPathCmd = &cobra.Command{
	Use: "rm [path]",
	Aliases: []string{
		"remove",
	},
	Short: "Remove bucket path contents",
	Long:  `Remove files and directories under a bucket path.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		if configViper.GetString("project") == "" {
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
