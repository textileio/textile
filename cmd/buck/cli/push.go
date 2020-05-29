package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/api/buckets/client"
	bucks "github.com/textileio/textile/buckets"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
)

var bucketPushCmd = &cobra.Command{
	Use:   "push",
	Short: "Push bucket object changes",
	Long:  `Pushes paths that have been added to and paths that have been removed or differ from the local bucket root.`,
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

		dbID := cmd.ThreadIDFromString(config.Viper.GetString("thread"))
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
			ctx, acancel := context.WithTimeout(context.Background(), cmd.Timeout)
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

		key := config.Viper.GetString("key")
		xr := buck.Path()
		var rm []change
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
		if len(rm) > 0 {
			for _, c := range rm {
				xr = rmFile(key, xr, c.Path, force)
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
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

	ctx, cancel := clients.Ctx.Thread(addFileTimeout)
	defer cancel()
	opts := []client.Option{client.WithProgress(progress)}
	if !force {
		opts = append(opts, client.WithFastForwardOnly(xroot))
	}
	added, root, err := clients.Buckets.PushPath(ctx, key, filePath, file, opts...)
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
	ctx, cancel := clients.Ctx.Thread(addFileTimeout)
	defer cancel()
	var opts []client.Option
	if !force {
		opts = append(opts, client.WithFastForwardOnly(xroot))
	}
	root, err := clients.Buckets.RemovePath(ctx, key, filePath, opts...)
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
