package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	pbar "github.com/cheggaaa/pb/v3"
	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/api/buckets/client"
	pb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/cmd"
)

var (
	errNotABucket = fmt.Errorf("not a bucket (or any of the parent directories): .textile")
)

func init() {
	rootCmd.AddCommand(bucketCmd)
	bucketCmd.AddCommand(initBucketPathCmd, lsBucketPathCmd, pushBucketPathCmd, pullBucketPathCmd, catBucketPathCmd, rmBucketPathCmd)

	initBucketPathCmd.PersistentFlags().String("key", "", "Bucket key")
	initBucketPathCmd.PersistentFlags().String("org", "", "Org username")
	initBucketPathCmd.PersistentFlags().Bool("public", false, "Allow public access")
	initBucketPathCmd.PersistentFlags().String("thread", "", "Thread ID")
	initBucketPathCmd.Flags().Bool("existing", false, "If set, initalizes from an existing remote Bucket")

	if err := cmd.BindFlags(configViper, initBucketPathCmd, flags); err != nil {
		cmd.Fatal(err)
	}
}

var bucketCmd = &cobra.Command{
	Use:   "bucket",
	Short: "Manage a bucket",
	Long:  `Init a bucket and push and pull files and folders.`,
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(configViper, flags)
		if configViper.ConfigFileUsed() == "" {
			cmd.Fatal(errNotABucket)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		lsBucketPath(args)
	},
}

var initBucketPathCmd = &cobra.Command{
	Use:   "init",
	Short: "Create an empty bucket",
	Long: `Create an empty bucket.

A .textile directory and config file will be created in the current working directory.
Existing configs will not be overwritten.
`,
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(configViper, flags)
	},
	Run: func(c *cobra.Command, args []string) {
		root, err := os.Getwd()
		if err != nil {
			cmd.Fatal(err)
		}

		dir := filepath.Join(root, ".textile")
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			cmd.Fatal(err)
		}
		filename := filepath.Join(dir, "config.yml")
		if _, err := os.Stat(filename); err == nil {
			cmd.Fatal(fmt.Errorf("bucket %s is already initialized", root))
		}

		existing, err := c.Flags().GetBool("existing")
		if err != nil {
			cmd.Fatal(err)
		}

		if existing {
			ctx, cancel := authCtx(cmdTimeout)
			defer cancel()

			res, err := users.ListThreads(ctx)
			if err != nil {
				cmd.Fatal(err)
			}

			type bucketInfo struct {
				ID   thread.ID
				Name string
				Key  string
			}
			var bucketInfos []bucketInfo
			for _, reply := range res.List {
				id, err := thread.Cast(reply.ID)
				if err != nil {
					cmd.Fatal(err)
				}
				ctx = common.NewThreadIDContext(ctx, id)
				res, err := buckets.List(ctx)
				if err != nil {
					cmd.Fatal(err)
				}
				for _, root := range res.Roots {
					name := "unnamed"
					if root.Name != "" {
						name = root.Name
					}
					bucketInfos = append(bucketInfos, bucketInfo{ID: id, Name: name, Key: root.Key})
				}
			}

			prompt := promptui.Select{
				Label: "Which exiting Bucket do you want to init from?",
				Items: bucketInfos,
				Templates: &promptui.SelectTemplates{
					Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .Name | bold }} {{ .Key | faint | bold }}`, promptui.IconSelect),
					Inactive: `{{ .Name | faint }} {{ .Key | faint | bold }}`,
					Selected: aurora.Sprintf(aurora.BrightBlack("> Selected Bucket {{ .Name | white | bold }}")),
				},
			}
			index, _, err := prompt.Run()
			if err != nil {
				cmd.Fatal(err)
			}

			selected := bucketInfos[index]

			configViper.Set("thread", selected.ID.String())
			configViper.Set("key", selected.Key)
		}

		var dbID thread.ID
		xthread := configViper.GetString("thread")
		if configViper.GetString("thread") != "" {
			var err error
			dbID, err = thread.Decode(xthread)
			if err != nil {
				cmd.Fatal(fmt.Errorf("invalid thread ID"))
			}
		}

		xkey := configViper.GetString("key")
		initRemote := true
		if xkey != "" {
			if !dbID.Defined() {
				cmd.Fatal(fmt.Errorf("the --thread flag is required when using --key"))
			}
			initRemote = false
		}

		var name string
		if initRemote {
			prompt := promptui.Prompt{
				Label: "Enter a name for your new bucket (optional)",
			}
			var err error
			name, err = prompt.Run()
			if err != nil {
				cmd.End("")
			}
		}

		if !dbID.Defined() {
			selected := selectThread("Buckets are written to a thread. Select an existing thread or create a new one", aurora.Sprintf(
				aurora.BrightBlack("> Selected thread {{ .ID | white | bold }}")))
			if selected.ID == "Create new" {
				ctx, cancel := threadCtx(cmdTimeout)
				defer cancel()
				dbID = thread.NewIDV1(thread.Raw, 32)
				if err := threads.NewDB(ctx, dbID); err != nil {
					cmd.Fatal(err)
				}
			} else {
				var err error
				dbID, err = thread.Decode(selected.ID)
				if err != nil {
					cmd.Fatal(err)
				}
			}
			configViper.Set("thread", dbID.String())
		}

		if initRemote {
			ctx, cancel := threadCtx(cmdTimeout)
			defer cancel()
			buck, err := buckets.Init(ctx, name)
			if err != nil {
				cmd.Fatal(err)
			}
			configViper.Set("key", buck.Root.Key)

			cmd.Message("Your bucket links:")
			cmd.Message("%s Thread link", aurora.White(buck.URL).Bold())
			if buck.WWW != "" {
				cmd.Message("%s Bucket website", aurora.White(buck.WWW).Bold())
			}
			cmd.Message("%s IPNS website (propagation can be slow)", aurora.White(buck.IPNS).Bold())
		}

		if err := configViper.WriteConfigAs(filename); err != nil {
			cmd.Fatal(err)
		}
		if existing {
			key := configViper.GetString("key")
			count := getPath(key, ".", filepath.Dir("."), ".")
			cmd.Success("Initialized from remote and pulled %d files to %s", aurora.White(count).Bold(), aurora.White(root).Bold())
		} else {
			cmd.Success("Initialized an empty bucket in %s", aurora.White(root).Bold())
		}
	},
}

var lsBucketPathCmd = &cobra.Command{
	Use: "ls [path]",
	Aliases: []string{
		"list",
	},
	Short: "List bucket path contents",
	Long:  `List files and directories under a bucket path.`,
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(configViper, flags)
		if configViper.ConfigFileUsed() == "" {
			cmd.Fatal(errNotABucket)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		lsBucketPath(args)
	},
}

func lsBucketPath(args []string) {
	ctx, cancel := threadCtx(cmdTimeout)
	defer cancel()

	var pth string
	if len(args) > 0 {
		pth = args[0]
	}
	if pth == "." || pth == "/" || pth == "./" {
		pth = ""
	}
	key := configViper.GetString("key")
	rep, err := buckets.ListPath(ctx, key, pth)
	if err != nil {
		cmd.Fatal(err)
	}
	var items []*pb.ListPathReply_Item
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
	Short: "Push to a bucket path (interactive)",
	Long: `Push files and directories to a bucket path. Existing paths will be overwritten. Non-existing paths will be created.

Using the '--org' flag will create a new bucket under the organization's account.

File structure is mirrored in the bucket. For example, given the directory:
    foo/one.txt
    foo/bar/two.txt
    foo/bar/baz/three.txt

These 'push' commands result in the following bucket structures.

'tt bucket push foo/ .':
    one.txt
    bar/two.txt
    bar/baz/three.txt
		
'tt bucket push foo mybuck':
    mybuck/foo/one.txt
    mybuck/foo/bar/two.txt
    mybuck/foo/bar/baz/three.txt

'tt bucket push foo/bar mybuck':
    mybuck/bar/two.txt
    mybuck/bar/baz/three.txt

'tt bucket push foo/bar/baz mybuck':
    mybuck/baz/three.txt

'tt bucket push foo/bar/baz/three.txt mybuck':
    mybuck/three.txt
`,
	Args: cobra.MinimumNArgs(2),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(configViper, flags)
	},
	Run: func(c *cobra.Command, args []string) {
		conf := configViper.ConfigFileUsed()
		if conf == "" {
			cmd.Fatal(errNotABucket)
		}
		root := filepath.Dir(filepath.Dir(conf))

		dbID := getThreadID()
		if !dbID.Defined() {
			cmd.Fatal(fmt.Errorf("thread is not defined"))
		}

		var names []string
		var paths []string
		bucketPath, args := args[len(args)-1], args[:len(args)-1]
		for _, a := range args {
			abs, err := filepath.Abs(a)
			if err != nil {
				cmd.Fatal(err)
			}
			if !strings.HasPrefix(abs, root) {
				cmd.Fatal(fmt.Errorf("the path %s is not under the current bucket root", abs))
			}

			dir := filepath.Dir(a)
			if err := filepath.Walk(a, func(n string, info os.FileInfo, err error) error {
				if err != nil {
					cmd.Fatal(err)
				}
				if !info.IsDir() {
					if strings.HasSuffix(n, ".DS_Store") {
						return nil
					}
					names = append(names, n)
					var p string
					if n == a { // This is a file given as an arg
						// In this case, the bucket path should not include the directory
						p = filepath.Join(bucketPath, info.Name())
					} else { // This is a directory given as an arg, or one of its sub directories
						// The bucket path should maintain directory structure
						if dir != "." {
							n = strings.TrimPrefix(n, dir)
						}
						p = filepath.Join(bucketPath, n)
					}
					paths = append(paths, p)
				}
				return nil
			}); err != nil {
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

		key := configViper.GetString("key")
		for i := range names {
			addFile(key, names[i], paths[i])
		}
		cmd.Success("Pushed %d files to %s", aurora.White(len(names)).Bold(), aurora.White(bucketPath).Bold())
	},
}

func addFile(key, name, filePath string) {
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
		ctx, cancel := threadCtx(addFileTimeout)
		defer cancel()
		if _, _, err = buckets.PushPath(ctx, key, filePath, file, client.WithProgress(progress)); err != nil {
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

'tt bucket pull foo mydir':
    mydir/foo/one.txt
    mydir/foo/bar/two.txt
    mydir/foo/bar/baz/three.txt

'tt bucket pull foo/bar mydir':
    mydir/bar/two.txt
    mydir/bar/baz/three.txt

'tt bucket pull foo/bar/baz mydir':
    mydir/baz/three.txt

'tt bucket pull foo/bar/baz/three.txt mydir':
    mydir/three.txt

'tt bucket pull foo .':
    foo/one.txt
    foo/bar/two.txt
    foo/bar/baz/three.txt
`,
	Args: cobra.ExactArgs(2),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(configViper, flags)
		if configViper.ConfigFileUsed() == "" {
			cmd.Fatal(errNotABucket)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		conf := configViper.ConfigFileUsed()
		if conf == "" {
			cmd.Fatal(errNotABucket)
		}
		root := filepath.Dir(filepath.Dir(conf))
		abs, err := filepath.Abs(args[1])
		if err != nil {
			cmd.Fatal(err)
		}
		if !strings.HasPrefix(abs, root) {
			cmd.Fatal(fmt.Errorf("the path %s is not under the current bucket root", abs))
		}

		key := configViper.GetString("key")
		count := getPath(key, args[0], filepath.Dir(args[0]), args[1])
		cmd.Success("Pulled %d files to %s", aurora.White(count).Bold(), aurora.White(args[1]).Bold())
	},
}

func getPath(key, pth, dir, dest string) (count int) {
	ctx, cancel := threadCtx(cmdTimeout)
	defer cancel()
	rep, err := buckets.ListPath(ctx, key, pth)
	if err != nil {
		cmd.Fatal(err)
	}

	if rep.Item.IsDir {
		for _, i := range rep.Item.Items {
			count += getPath(key, filepath.Join(pth, filepath.Base(i.Path)), dir, dest)
		}
	} else {
		name := filepath.Join(dest, strings.TrimPrefix(pth, dir))
		getFile(key, pth, name, rep.Item.Size)
		count++
	}
	return count
}

func getFile(key, filePath, name string, size int64) {
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
		ctx, cancel := threadCtx(getFileTimeout)
		defer cancel()
		if err = buckets.PullPath(ctx, key, filePath, file, client.WithProgress(progress)); err != nil {
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
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(configViper, flags)
		if configViper.ConfigFileUsed() == "" {
			cmd.Fatal(errNotABucket)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := threadCtx(getFileTimeout)
		defer cancel()
		key := configViper.GetString("key")
		if err := buckets.PullPath(ctx, key, args[0], os.Stdout); err != nil {
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
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(configViper, flags)
		if configViper.ConfigFileUsed() == "" {
			cmd.Fatal(errNotABucket)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := threadCtx(cmdTimeout)
		defer cancel()
		key := configViper.GetString("key")
		if err := buckets.RemovePath(ctx, key, args[0]); err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("Removed %s", aurora.White(args[0]).Bold())
	},
}
