package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/api/buckets/client"
	pb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/api/common"
	bucks "github.com/textileio/textile/buckets"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/uiprogress"
)

const nonFastForwardMsg = "the root of your bucket is behind (try `%s` before pushing again)"

var errNotABucket = fmt.Errorf("not a bucket (or any of the parent directories): .textile")

func init() {
	rootCmd.AddCommand(bucketCmd)
	bucketCmd.AddCommand(bucketInitCmd, bucketLinksCmd, bucketRootCmd, bucketStatusCmd, bucketLsCmd, bucketPushCmd, bucketPullCmd, bucketCatCmd, bucketDestroyCmd, bucketArchiveCmd)
	bucketArchiveCmd.AddCommand(bucketArchiveStatusCmd, bucketArchiveInfoCmd)

	bucketInitCmd.PersistentFlags().String("key", "", "Bucket key")
	bucketInitCmd.PersistentFlags().String("org", "", "Org username")
	bucketInitCmd.PersistentFlags().Bool("public", false, "Allow public access")
	bucketInitCmd.PersistentFlags().String("thread", "", "Thread ID")
	bucketInitCmd.Flags().BoolP("existing", "e", false, "Initializes from an existing remote bucket if true")
	if err := cmd.BindFlags(configViper, bucketInitCmd, flags); err != nil {
		cmd.Fatal(err)
	}

	bucketPushCmd.Flags().BoolP("force", "f", false, "Allows non-fast-forward updates if true")
	bucketPushCmd.Flags().BoolP("yes", "y", false, "Skips the confirmation prompt if true")

	bucketPullCmd.Flags().BoolP("force", "f", false, "Force pull all remote files if true")
	bucketPullCmd.Flags().Bool("sync", false, "Pulls and prunes local changes if true")
	bucketPullCmd.Flags().BoolP("yes", "y", false, "Skips the confirmation prompt if true")

	bucketArchiveStatusCmd.Flags().BoolP("watch", "w", false, "Watch execution log")

	uiprogress.Empty = ' '
	uiprogress.Fill = '-'
}

var bucketCmd = &cobra.Command{
	Use: "bucket",
	Aliases: []string{
		"buck",
	},
	Short: "Manage an object storage bucket",
	Long:  `Manages files and folders in an object storage bucket.`,
	Args:  cobra.ExactArgs(0),
}

var bucketInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new or existing bucket",
	Long: `Initializes a new or existing bucket.

A .textile config directory and a seed file will be created in the current working directory.
Existing configs will not be overwritten.

Use the '--existing' flag to initialize from an existing remote bucket.
`,
	Args: cobra.ExactArgs(0),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(configViper, flags)
	},
	Run: func(c *cobra.Command, args []string) {
		root, err := os.Getwd()
		if err != nil {
			cmd.Fatal(err)
		}
		dir := filepath.Join(root, configDir)
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
			bucketInfos := make([]bucketInfo, 0)
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
				Label: "Which exiting bucket do you want to init from?",
				Items: bucketInfos,
				Templates: &promptui.SelectTemplates{
					Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .Name | bold }} {{ .Key | faint | bold }}`, promptui.IconSelect),
					Inactive: `{{ .Name | faint }} {{ .Key | faint | bold }}`,
					Selected: aurora.Sprintf(aurora.BrightBlack("> Selected bucket {{ .Name | white | bold }}")),
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
			selected := selectThread("Buckets are written to a threadDB. Select or create a new one", aurora.Sprintf(
				aurora.BrightBlack("> Selected threadDB {{ .ID | white | bold }}")), true)
			if selected.ID == "Create new" {
				if selected.Name == "" {
					prompt := promptui.Prompt{
						Label: "Enter a name for your new threadDB (optional)",
					}
					var err error
					selected.Name, err = prompt.Run()
					if err != nil {
						cmd.End("")
					}
				}
				ctx, cancel := threadCtx(cmdTimeout)
				defer cancel()
				dbID = thread.NewIDV1(thread.Raw, 32)
				ctx = common.NewThreadNameContext(ctx, selected.Name)
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
			rep, err := buckets.Init(ctx, name)
			if err != nil {
				cmd.Fatal(err)
			}
			configViper.Set("key", rep.Root.Key)

			seed := filepath.Join(root, bucks.SeedName)
			file, err := os.Create(seed)
			if err != nil {
				cmd.Fatal(err)
			}
			_, err = file.Write(rep.Seed)
			if err != nil {
				file.Close()
				cmd.Fatal(err)
			}
			file.Close()

			buck, err := local.NewBucket(root, options.BalancedLayout)
			if err != nil {
				cmd.Fatal(err)
			}
			actx, acancel := context.WithTimeout(context.Background(), cmdTimeout)
			defer acancel()
			if err = buck.ArchiveFile(actx, seed, bucks.SeedName); err != nil {
				cmd.Fatal(err)
			}

			printLinks(rep.Links)
		}

		if err := configViper.WriteConfigAs(filename); err != nil {
			cmd.Fatal(err)
		}
		if initRemote {
			cmd.Success("Initialized an empty bucket in %s", aurora.White(root).Bold())
		} else {
			key := configViper.GetString("key")
			count := getPath(key, "", root, nil, false)

			buck, err := local.NewBucket(root, options.BalancedLayout)
			if err != nil {
				cmd.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
			defer cancel()
			if err = buck.Archive(ctx); err != nil {
				cmd.Fatal(err)
			}
			cmd.Success("Initialized from remote and pulled %d objects to %s", aurora.White(count).Bold(), aurora.White(root).Bold())
		}
	},
}

func printLinks(reply *pb.LinksReply) {
	cmd.Message("Your bucket links:")
	cmd.Message("%s Thread link", aurora.White(reply.URL).Bold())
	cmd.Message("%s IPNS link (propagation can be slow)", aurora.White(reply.IPNS).Bold())
	if reply.WWW != "" {
		cmd.Message("%s Bucket website", aurora.White(reply.WWW).Bold())
	}
}

var bucketLinksCmd = &cobra.Command{
	Use: "links",
	Aliases: []string{
		"link",
	},
	Short: "Show links to where this bucket can be accessed",
	Long:  `Displays a thread, IPNS, and website link to this bucket.`,
	Args:  cobra.ExactArgs(0),
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
		reply, err := buckets.Links(ctx, key)
		if err != nil {
			cmd.Fatal(err)
		}
		printLinks(reply)
	},
}

var bucketRootCmd = &cobra.Command{
	Use:   "root",
	Short: "Show local bucket root CID",
	Long:  `Shows the local bucket root CID`,
	Args:  cobra.ExactArgs(0),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(configViper, flags)
	},
	Run: func(c *cobra.Command, args []string) {
		conf := configViper.ConfigFileUsed()
		if conf == "" {
			cmd.Fatal(errNotABucket)
		}
		root := filepath.Dir(filepath.Dir(conf))
		buck, err := local.NewBucket(root, options.BalancedLayout)
		if err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("%s", aurora.White(buck.Path().Cid()).Bold())
	},
}

var bucketStatusCmd = &cobra.Command{
	Use: "status",
	Aliases: []string{
		"st",
	},
	Short: "Show bucket object changes",
	Long:  `Displays paths that have been added to and paths that have been removed or differ from the local bucket root.`,
	Args:  cobra.ExactArgs(0),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(configViper, flags)
	},
	Run: func(c *cobra.Command, args []string) {
		conf := configViper.ConfigFileUsed()
		if conf == "" {
			cmd.Fatal(errNotABucket)
		}
		root := filepath.Dir(filepath.Dir(conf))
		buck, err := local.NewBucket(root, options.BalancedLayout)
		if err != nil {
			cmd.Fatal(err)
		}
		diff := getDiff(buck, root)
		if len(diff) == 0 {
			cmd.End("Everything up-to-date")
		}
		for _, c := range diff {
			cf := changeColor(c.Type)
			cmd.Message("%s  %s", cf(changeType(c.Type)), cf(c.Rel))
		}
	},
}

var bucketLsCmd = &cobra.Command{
	Use: "ls [path]",
	Aliases: []string{
		"list",
	},
	Short: "List top-level or nested bucket objects",
	Long:  `Lists top-level or nested bucket objects.`,
	Args:  cobra.MaximumNArgs(1),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(configViper, flags)
		if configViper.ConfigFileUsed() == "" {
			cmd.Fatal(errNotABucket)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		var pth string
		if len(args) > 0 {
			pth = args[0]
		}
		if pth == "." || pth == "/" || pth == "./" {
			pth = ""
		}

		ctx, cancel := threadCtx(cmdTimeout)
		defer cancel()
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
		if len(items) > 0 && !strings.HasPrefix(pth, configDir) {
			for _, item := range items {
				if item.Name == configDir {
					continue
				}
				var links string
				if item.IsDir {
					links = strconv.Itoa(len(item.Items))
				} else {
					links = "n/a"
				}
				data = append(data, []string{
					item.Name,
					strconv.Itoa(int(item.Size)),
					strconv.FormatBool(item.IsDir),
					links,
					item.Path,
				})
			}
		}

		if len(data) > 0 {
			cmd.RenderTable([]string{"name", "size", "dir", "objects", "path"}, data)
		}
		cmd.Message("Found %d objects", aurora.White(len(data)).Bold())
	},
}

var bucketPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push bucket object changes",
	Long:  `Pushes paths that have been added to and paths that have been removed or differ from the local bucket root.`,
	Args:  cobra.ExactArgs(0),
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

		buck, err := local.NewBucket(root, options.BalancedLayout)
		if err != nil {
			cmd.Fatal(err)
		}
		diff := getDiff(buck, root)
		force, err := c.Flags().GetBool("force")
		if err != nil {
			cmd.Fatal(err)
		}
		if force {
			// Reset the archive to just the seed file
			seed := filepath.Join(root, bucks.SeedName)
			ctx, acancel := context.WithTimeout(context.Background(), cmdTimeout)
			defer acancel()
			if err = buck.ArchiveFile(ctx, seed, bucks.SeedName); err != nil {
				cmd.Fatal(err)
			}
			// Add unique additions
		loop:
			for _, c := range getDiff(buck, root) {
				for _, x := range diff {
					if c.Path == x.Path {
						continue loop
					}
				}
				diff = append(diff, c)
			}
		}
		if len(diff) == 0 {
			cmd.End("Everything up-to-date")
		}

		yes, err := c.Flags().GetBool("yes")
		if err != nil {
			cmd.Fatal(err)
		}
		if !yes {
			for _, c := range diff {
				cf := changeColor(c.Type)
				cmd.Message("%s  %s", cf(changeType(c.Type)), cf(c.Rel))
			}
			prompt := promptui.Prompt{
				Label:     fmt.Sprintf("Push %d changes", len(diff)),
				IsConfirm: true,
			}
			if _, err := prompt.Run(); err != nil {
				cmd.End("")
			}
		}

		key := configViper.GetString("key")
		xr := buck.Path()
		var rm []change
		if len(diff) > 0 {
			startProgress()
			for _, c := range diff {
				switch c.Type {
				case dagutils.Mod, dagutils.Add:
					xr = addFile(key, xr, c.Rel, c.Path, force)
				case dagutils.Remove:
					rm = append(rm, c)
				}
			}
			stopProgress()
		}
		if len(rm) > 0 {
			for _, c := range rm {
				xr = rmFile(key, xr, c.Path, force)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err = buck.Archive(ctx); err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("%s", aurora.White(buck.Path().Cid()).Bold())
	},
}

func addFile(key string, xroot path.Resolved, name, filePath string, force bool) path.Resolved {
	file, err := os.Open(name)
	if err != nil {
		cmd.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		cmd.Fatal(err)
	}

	bar := addBar(filePath, info.Size())
	progress := make(chan int64)
	go func() {
		for up := range progress {
			if err := bar.Set(int(up)); err != nil {
				cmd.Fatal(err)
			}
		}
	}()

	ctx, cancel := threadCtx(addFileTimeout)
	defer cancel()
	opts := []client.Option{client.WithProgress(progress)}
	if !force {
		opts = append(opts, client.WithFastForwardOnly(xroot))
	}
	added, root, err := buckets.PushPath(ctx, key, filePath, file, opts...)
	if err != nil {
		if strings.HasSuffix(err.Error(), bucks.ErrNonFastForward.Error()) {
			cmd.Fatal(errors.New(nonFastForwardMsg), aurora.Cyan("tt bucket pull"))
		} else {
			cmd.Fatal(err)
		}
	} else {
		finishBar(bar, filePath, added.Cid())
	}
	return root
}

func rmFile(key string, xroot path.Resolved, filePath string, force bool) path.Resolved {
	ctx, cancel := threadCtx(addFileTimeout)
	defer cancel()
	var opts []client.Option
	if !force {
		opts = append(opts, client.WithFastForwardOnly(xroot))
	}
	root, err := buckets.RemovePath(ctx, key, filePath, opts...)
	if err != nil {
		if strings.HasSuffix(err.Error(), bucks.ErrNonFastForward.Error()) {
			cmd.Fatal(errors.New(nonFastForwardMsg), aurora.Cyan("tt bucket pull"))
		} else if !strings.HasSuffix(err.Error(), "no link by that name") {
			cmd.Fatal(err)
		}
	}
	fmt.Println("- " + filePath)
	return root
}

var bucketPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull bucket object changes",
	Long:  `Pulls paths that have been added to and paths that have been removed or differ from the remote bucket root.`,
	Args:  cobra.ExactArgs(0),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(configViper, flags)
	},
	Run: func(c *cobra.Command, args []string) {
		conf := configViper.ConfigFileUsed()
		if conf == "" {
			cmd.Fatal(errNotABucket)
		}
		root := filepath.Dir(filepath.Dir(conf))

		buck, err := local.NewBucket(root, options.BalancedLayout)
		if err != nil {
			cmd.Fatal(err)
		}
		diff := getDiff(buck, root)

		sync, err := c.Flags().GetBool("sync")
		if err != nil {
			cmd.Fatal(err)
		}
		yes, err := c.Flags().GetBool("yes")
		if err != nil {
			cmd.Fatal(err)
		}
		if !yes && sync && len(diff) > 0 {
			for _, c := range diff {
				cf := changeColor(c.Type)
				cmd.Message("%s  %s", cf(changeType(c.Type)), cf(c.Rel))
			}
			prompt := promptui.Prompt{
				Label:     fmt.Sprintf("Discard %d local changes", len(diff)),
				IsConfirm: true,
			}
			if _, err := prompt.Run(); err != nil {
				cmd.End("")
			}
		}

		// Tmp move local modifications and additions if not syncing
		if !sync {
			for _, c := range diff {
				switch c.Type {
				case dagutils.Mod, dagutils.Add:
					if err := os.Rename(c.Rel, c.Rel+".buckpatch"); err != nil {
						cmd.Fatal(err)
					}
				}
			}
		}

		force, err := c.Flags().GetBool("force")
		if err != nil {
			cmd.Fatal(err)
		}
		key := configViper.GetString("key")
		count := getPath(key, "", root, buck, force)
		if count == 0 {
			cmd.End("Everything up-to-date")
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err = buck.Archive(ctx); err != nil {
			cmd.Fatal(err)
		}

		// Re-apply local changes if not syncing
		if !sync {
			for _, c := range diff {
				switch c.Type {
				case dagutils.Mod, dagutils.Add:
					if err := os.Rename(c.Rel+".buckpatch", c.Rel); err != nil {
						cmd.Fatal(err)
					}
				case dagutils.Remove:
					if err := os.Remove(c.Rel); err != nil {
						cmd.Fatal(err)
					}
				}
			}
		}
		cmd.Message("%s", aurora.White(buck.Path().Cid()).Bold())
	},
}

func getPath(key, pth, root string, buck *local.Bucket, force bool) (count int) {
	all, missing := listPath(key, pth, root, buck, force)
	count = len(missing)
	var rm []string
	list := walkPath(root)
loop:
	for _, n := range list {
		for _, r := range all {
			if n == r.name {
				continue loop
			}
		}
		rm = append(rm, n)
	}
	count += len(rm)
	if count == 0 {
		return
	}

	if len(missing) > 0 {
		var wg sync.WaitGroup
		startProgress()
		for _, o := range missing {
			wg.Add(1)
			go func(o object) {
				defer wg.Done()
				getFile(key, o.path, o.name, o.size, o.cid)
			}(o)
		}
		wg.Wait()
		stopProgress()
	}
	if len(rm) > 0 {
		for _, r := range rm {
			if err := os.Remove(r); err != nil {
				cmd.Fatal(err)
			}
			fmt.Println("- " + strings.TrimPrefix(r, root+"/"))
		}
	}
	return count
}

type object struct {
	path string
	name string
	cid  cid.Cid
	size int64
}

func listPath(key, pth, dest string, buck *local.Bucket, force bool) (all, missing []object) {
	ctx, cancel := threadCtx(cmdTimeout)
	defer cancel()
	rep, err := buckets.ListPath(ctx, key, pth)
	if err != nil {
		cmd.Fatal(err)
	}
	if rep.Item.IsDir {
		for _, i := range rep.Item.Items {
			a, m := listPath(key, filepath.Join(pth, filepath.Base(i.Path)), dest, buck, force)
			all = append(all, a...)
			missing = append(missing, m...)
		}
	} else {
		name := filepath.Join(dest, pth)
		c, err := cid.Decode(rep.Item.Cid)
		if err != nil {
			cmd.Fatal(err)
		}
		o := object{path: pth, name: name, size: rep.Item.Size, cid: c}
		all = append(all, o)
		if !force && buck != nil {
			c, err := cid.Decode(rep.Item.Cid)
			if err != nil {
				cmd.Fatal(err)
			}
			lc, err := buck.HashFile(name)
			if err == nil && lc.Equals(c) { // File exists, skip it
				return
			}
		}
		missing = append(missing, o)
	}
	return all, missing
}

func getFile(key, filePath, name string, size int64, c cid.Cid) {
	if err := os.MkdirAll(filepath.Dir(name), os.ModePerm); err != nil {
		cmd.Fatal(err)
	}
	file, err := os.Create(name)
	if err != nil {
		cmd.Fatal(err)
	}
	defer file.Close()

	bar := addBar(filePath, size)
	progress := make(chan int64)
	go func() {
		for up := range progress {
			if err := bar.Set(int(up)); err != nil {
				cmd.Fatal(err)
			}
		}
	}()

	ctx, cancel := threadCtx(getFileTimeout)
	defer cancel()
	if err := buckets.PullPath(ctx, key, filePath, file, client.WithProgress(progress)); err != nil {
		cmd.Fatal(err)
	}
	finishBar(bar, filePath, c)
}

var bucketCatCmd = &cobra.Command{
	Use:   "cat [path]",
	Short: "Cat bucket objects at path",
	Long:  `Cats bucket objects at path.`,
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

var bucketDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy bucket and all associated data",
	Long:  `Destroys the bucket and all associated data.`,
	Args:  cobra.ExactArgs(0),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(configViper, flags)
	},
	Run: func(c *cobra.Command, args []string) {
		conf := configViper.ConfigFileUsed()
		if conf == "" {
			cmd.Fatal(errNotABucket)
		}
		root := filepath.Dir(filepath.Dir(conf))

		cmd.Warn("%s", aurora.Red("This action cannot be undone. The bucket and all associated data will be permanently deleted."))
		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Are you absolutely sure"),
			IsConfirm: true,
		}
		if _, err := prompt.Run(); err != nil {
			cmd.End("")
		}

		ctx, cancel := threadCtx(cmdTimeout)
		defer cancel()
		key := configViper.GetString("key")
		if err := buckets.Remove(ctx, key); err != nil {
			cmd.Fatal(err)
		}

		_ = os.RemoveAll(filepath.Join(root, bucks.SeedName))
		_ = os.RemoveAll(filepath.Join(root, configDir))
		cmd.Success("Your bucket has been deleted")
	},
}

var bucketArchiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Create a Filecoin bucket archive",
	Long:  `Creates a Filecoin bucket archive from the remote root.`,
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
		if _, err := buckets.Archive(ctx, key); err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("Archive queued successfully")
	},
}

var bucketArchiveStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of the last archive",
	Long:  `Shows the status of the most recent bucket archive.`,
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
		r, err := buckets.ArchiveStatus(ctx, key)
		if err != nil {
			cmd.Fatal(err)
		}
		switch r.GetStatus() {
		case pb.ArchiveStatusReply_Failed:
			cmd.Warn("Archive failed with message: %s", r.GetFailedMsg())
		case pb.ArchiveStatusReply_Canceled:
			cmd.Warn("Archive was superseded by a new executing archive")
		case pb.ArchiveStatusReply_Executing:
			cmd.Message("Archive is currently executing, grab a coffee and be patient...")
		case pb.ArchiveStatusReply_Done:
			cmd.Success("Archive executed successfully!")
		default:
			cmd.Warn("Archive status unknown")
		}
		watch, err := c.Flags().GetBool("watch")
		if err != nil {
			cmd.Fatal(err)
		}
		if watch {
			fmt.Printf("\n")
			cmd.Message("Cid logs:")
			ch := make(chan string)
			wCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			go func() {
				err = buckets.ArchiveWatch(wCtx, key, ch)
				close(ch)
			}()
			for msg := range ch {
				cmd.Message("\t %s", msg)
				r, err := buckets.ArchiveStatus(ctx, key)
				if err != nil {
					cmd.Fatal(err)
				}
				if isJobStatusFinal(r.GetStatus()) {
					cancel()
				}
			}
			if err != nil {
				cmd.Fatal(err)
			}
		}
	},
}

func isJobStatusFinal(status pb.ArchiveStatusReply_Status) bool {
	switch status {
	case pb.ArchiveStatusReply_Failed, pb.ArchiveStatusReply_Canceled, pb.ArchiveStatusReply_Done:
		return true
	case pb.ArchiveStatusReply_Executing:
		return false
	}
	cmd.Fatal(fmt.Errorf("unknown job status"))
	return true

}

var bucketArchiveInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show info about the current archive",
	Long:  `Shows information about the current archive.`,
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
		r, err := buckets.ArchiveInfo(ctx, key)
		if err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("Archive of Cid %s has %d deals:\n", r.Archive.Cid, len(r.Archive.Deals))
		var data [][]string
		for _, d := range r.Archive.GetDeals() {
			data = append(data, []string{d.ProposalCid, d.Miner})
		}
		cmd.RenderTable([]string{"ProposalCid", "Miner"}, data)

	},
}
