package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	du "github.com/ipfs/go-merkledag/dagutils"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/buckets"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
)

var remoteCmd = &cobra.Command{
	Use:   "remote",
	Short: "Add a file or folder to and remove remote bucket paths",
	Long: `Adds files and folders to and removes remote bucket paths.

Note: When using this command, the local bucket repo is ignored.`,
	Args: cobra.ExactArgs(0),
}

var remoteAddCmd = &cobra.Command{
	Use:   "add [file/folder] [path]",
	Short: "Add a file or folder to a remote bucket path",
	Long: `Adds a file or folder to a remote bucket path.
`,
	Args: cobra.ExactArgs(2),
	Run: func(c *cobra.Command, args []string) {
		yes, err := c.Flags().GetBool("yes")
		cmd.ErrCheck(err)
		quiet, err := c.Flags().GetBool("quiet")
		cmd.ErrCheck(err)
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.PushTimeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)

		var events chan local.Event
		if !quiet {
			events = make(chan local.Event)
			defer close(events)
			go handleEvents(events)
		}

		var (
			src   = filepath.Clean(args[0])
			pth   = filepath.Clean(args[1])
			names []string
			diff  []local.Change
		)

		err = filepath.Walk(src, func(n string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				f := strings.TrimPrefix(n, pth+string(os.PathSeparator))
				if local.Ignore(n) ||
					f == buckets.SeedName ||
					strings.HasPrefix(f, buck.ConfDir()) ||
					strings.HasSuffix(f, local.PatchExt) {
					return nil
				}
				names = append(names, n)
			}
			return nil
		})
		cmd.ErrCheck(err)

		for _, n := range names {
			r, err := filepath.Rel(buck.Cwd(), n)
			cmd.ErrCheck(err)
			p := filepath.Join(args[1], filepath.Base(n))
			diff = append(diff, local.Change{Type: du.Add, Name: n, Path: p, Rel: r})
		}

		confirm := getConfirm("Push %d changes", yes)
		if confirm != nil {
			if ok := confirm(diff); !ok {
				cmd.End("")
			}
		}

		ctx, err = buck.Context(ctx)
		cmd.ErrCheck(err)

		r, err := buck.AddRemoteFiles(ctx, buck.Key(), nil, diff, true, events)
		if errors.Is(err, local.ErrAborted) {
			cmd.End("")
		} else if err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("%s", aurora.White(r.Cid()).Bold())
	},
}

var remoteRemoveCmd = &cobra.Command{
	Use: "rm [path]",
	Aliases: []string{
		"remove",
	},
	Short: "Remove a path from a remote bucket",
	Long: `Removes a path from a remote bucket.
`,
	Args: cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		yes, err := c.Flags().GetBool("yes")
		cmd.ErrCheck(err)
		quiet, err := c.Flags().GetBool("quiet")
		cmd.ErrCheck(err)
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.PushTimeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)

		var events chan local.Event
		if !quiet {
			events = make(chan local.Event)
			defer close(events)
			go handleEvents(events)
		}

		pth := filepath.Clean(args[0])
		change := local.Change{
			Type: du.Remove,
			Name: pth,
			Path: pth,
			Rel:  pth,
		}

		confirm := getConfirm("Push %d changes", yes)
		if confirm != nil {
			if ok := confirm([]local.Change{change}); !ok {
				cmd.End("")
			}
		}

		ctx, err = buck.Context(ctx)
		cmd.ErrCheck(err)

		r, err := buck.RemoveRemoteFile(ctx, buck.Key(), nil, change, true, events)
		if errors.Is(err, local.ErrAborted) {
			cmd.End("")
		} else if err != nil {
			cmd.Fatal(err)
		}
		if r != nil {
			cmd.Message("%s", aurora.White(r.Cid()).Bold())
		}
	},
}
