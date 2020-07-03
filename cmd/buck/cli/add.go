package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/textileio/textile/api/buckets/client"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

const (
	promptSkip    = "Skip"
	promptMerge   = "Merge"
	promptReplace = "Replace"
)

var mergeStrategySelect = []string{promptSkip, promptMerge, promptReplace}

var bucketAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add adds a UnixFS DAG locally.",
	Long:  `Add adds a UnixFS DAG locally, merging with any existing content.`,
	Args:  cobra.ExactArgs(2),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(config.Viper, config.Flags)
	},
	Run: func(c *cobra.Command, args []string) {
		conf := config.Viper.ConfigFileUsed()
		if conf == "" {
			cmd.Fatal(errNotABucket)
		}

		cmdCid := args[0]
		targetCid, err := cid.Decode(cmdCid)
		if err != nil {
			cmd.Fatal(err)
		}
		dest := args[1]

		yes, err := c.Flags().GetBool("yes")
		if err != nil {
			cmd.Fatal(err)
		}

		if err := mergeIpfsPath(path.IpfsPath(targetCid), dest, yes); err != nil {
			cmd.Fatal(err)
		}
	},
}

func mergeIpfsPath(ipfsBasePth path.Path, dest string, overwriteAll bool) error {
	folderReplace, toAdd := listMergePath(ipfsBasePth, "", dest, overwriteAll)

	// Remove all the folders that were decided to be replaced.
	for _, fr := range folderReplace {
		if err := os.RemoveAll(fr); err != nil {
			return err
		}
	}

	// Add files that are missing, or were decided to be overwritten.
	if len(toAdd) > 0 {
		var wg sync.WaitGroup
		startProgress()
		for _, o := range toAdd {
			wg.Add(1)
			go func(o object) {
				defer wg.Done()
				if err := os.Remove(o.path); err != nil && !os.IsNotExist(err) {
					cmd.Fatal(err)
				}
				trimmedDest := strings.TrimLeft(o.path, dest)
				getIpfsFile(path.Join(ipfsBasePth, trimmedDest), o.path, o.size, o.cid)
			}(o)
		}
		wg.Wait()
		stopProgress()
	}
	return nil
}

// listMergePath walks the local bucket and the remote IPFS UnixFS DAG asking
// the client if wants to (replace, merge, ignore) matching folders, and if wants
// to (overwrite, ignore) matching files. Any non-matching files or folders in the
// IPFS UnixFS DAG will be added locally.
// The first return value is a slice of path of folders that were decided to be
// replaced completely (not merged). The second return value are a list of files
// that should be added locally. If one of them exist, can be understood that should
// be overwritten.
func listMergePath(ipfsBasePth path.Path, ipfsRelPath, dest string, overwriteAll bool) ([]string, []object) {
	// List remote IPFS UnixFS path level
	ctx, cancel := clients.Ctx.Thread(cmd.Timeout)
	defer cancel()
	rep, err := clients.Buckets.ListIpfsPath(ctx, path.Join(ipfsBasePth, ipfsRelPath))
	if err != nil {
		cmd.Fatal(err)
	}

	// If its a dir, ask if should be ignored, replaced, or merged.
	if rep.Item.IsDir {
		var replacedFolders []string
		var toAdd []object

		var folderExists bool

		localFolderPath := filepath.Join(dest, ipfsRelPath)
		if _, err := os.Stat(localFolderPath); err == nil {
			folderExists = true
		}

		if folderExists && !overwriteAll {
			prompt := promptui.Select{
				Label: fmt.Sprintf("Merge strategy for  %s", localFolderPath),
				Items: mergeStrategySelect,
			}
			_, result, err := prompt.Run()
			if err != nil {
				cmd.Fatal(err)
			}
			if result == promptSkip {
				return nil, nil
			}
			if result == promptReplace {
				replacedFolders = append(replacedFolders, localFolderPath)
				overwriteAll = true
			}
		}
		for _, i := range rep.Item.Items {
			nestFolderReplace, nestAdd := listMergePath(ipfsBasePth, filepath.Join(ipfsRelPath, i.Name), dest, overwriteAll)
			replacedFolders = append(replacedFolders, nestFolderReplace...)
			toAdd = append(toAdd, nestAdd...)
		}
		return replacedFolders, toAdd
	}

	// If it's a file, add it if not exist, or ask for  a decision if wants to be overwritten.
	pth := filepath.Join(dest, ipfsRelPath)
	if _, err := os.Stat(pth); err == nil && !overwriteAll {
		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Overwrite  %s", pth),
			IsConfirm: true,
		}
		if _, err := prompt.Run(); err != nil {
			return nil, nil
		}
	}
	if err != nil && os.IsNotExist(err) {
		cmd.Fatal(err)
	}

	c, err := cid.Decode(rep.Item.Cid)
	if err != nil {
		cmd.Fatal(err)
	}
	o := object{path: pth, name: rep.Item.Name, size: rep.Item.Size, cid: c}
	return nil, []object{o}
}

func getIpfsFile(ipfsPath path.Path, filePath string, size int64, c cid.Cid) {
	if err := os.MkdirAll(filepath.Dir(filePath), os.ModePerm); err != nil {
		cmd.Fatal(err)
	}
	file, err := os.Create(filePath)
	if err != nil {
		cmd.Fatal(err)
	}
	defer file.Close()

	bar := addBar(filePath, size)
	progress := make(chan int64)
	go func() {
		ctx, cancel := clients.Ctx.Thread(getFileTimeout)
		defer cancel()
		if err := clients.Buckets.PullIpfsPath(ctx, ipfsPath, file, client.WithProgress(progress)); err != nil {
			cmd.Fatal(err)
		}
	}()
	for up := range progress {
		if err := bar.Set(int(up)); err != nil {
			cmd.Fatal(err)
		}
	}
	if err := bar.Set(int(size)); err != nil {
		cmd.Fatal(err)
	}
	finishBar(bar, filePath, c)
}
