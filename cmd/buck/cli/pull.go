package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/api/buckets/client"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
)

var bucketPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull bucket object changes",
	Long:  `Pulls paths that have been added to and paths that have been removed or differ from the remote bucket root.`,
	Args:  cobra.ExactArgs(0),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(config.Viper, config.Flags)
	},
	Run: func(c *cobra.Command, args []string) {
		conf := config.Viper.ConfigFileUsed()
		if conf == "" {
			cmd.Fatal(errNotABucket)
		}
		root := filepath.Dir(filepath.Dir(conf))

		buck, err := local.NewBucket(root, options.BalancedLayout)
		if err != nil {
			cmd.Fatal(err)
		}
		diff := getDiff(buck, root)

		hard, err := c.Flags().GetBool("hard")
		if err != nil {
			cmd.Fatal(err)
		}
		yes, err := c.Flags().GetBool("yes")
		if err != nil {
			cmd.Fatal(err)
		}
		if !yes && hard && len(diff) > 0 {
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

		// Tmp move local modifications and additions if not pulling hard
		if !hard {
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
		key := config.Viper.GetString("key")
		count := getPath(key, "", root, buck, diff, force)
		if count == 0 {
			cmd.End("Everything up-to-date")
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		if err = buck.Archive(ctx); err != nil {
			cmd.Fatal(err)
		}

		// Re-apply local changes if not pulling hard
		if !hard {
			for _, c := range diff {
				switch c.Type {
				case dagutils.Mod, dagutils.Add:
					if err := os.Rename(c.Rel+".buckpatch", c.Rel); err != nil {
						cmd.Fatal(err)
					}
				case dagutils.Remove:
					// If the file was also deleted on the remote,
					// the local deletion will already have been handled by getPath.
					// So, we just ignore the error here.
					_ = os.Remove(c.Rel)
				}
			}
		}
		cmd.Message("%s", aurora.White(buck.Path().Cid()).Bold())
	},
}

func getPath(key, pth, root string, buck *local.Bucket, localdiff []change, force bool) (count int) {
	all, missing := listPath(key, pth, root, buck, force)
	count = len(missing)
	var rm []string
	list := walkPath(root)
loop:
	for _, n := range list {
		for _, r := range all {
			if r.name == n {
				continue loop
			}
		}
		rm = append(rm, n)
	}
looop:
	for _, l := range localdiff {
		for _, r := range all {
			if r.path == l.Path {
				continue looop
			}
		}
		rm = append(rm, l.Rel)
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
			// The file may have been modified locally, in which case it will have been moved to a patch.
			// So, we just ignore the error here.
			_ = os.Remove(r)
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
	ctx, cancel := clients.Ctx.Thread(cmd.Timeout)
	defer cancel()
	rep, err := clients.Buckets.ListPath(ctx, key, pth)
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

	ctx, cancel := clients.Ctx.Thread(getFileTimeout)
	defer cancel()
	if err := clients.Buckets.PullPath(ctx, key, filePath, file, client.WithProgress(progress)); err != nil {
		cmd.Fatal(err)
	}
	finishBar(bar, filePath, c)
}
