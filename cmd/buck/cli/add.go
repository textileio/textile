package cli

import (
	"context"

	"github.com/ipfs/go-cid"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
)

var addCmd = &cobra.Command{
	Use:   "add [cid] [path]",
	Short: "Adds a UnixFs DAG locally at path",
	Long:  `Adds a UnixFs DAG locally at path, merging with existing content.`,
	Args:  cobra.ExactArgs(2),
	Run: func(c *cobra.Command, args []string) {
		yes, err := c.Flags().GetBool("yes")
		cmd.ErrCheck(err)
		target, err := cid.Decode(args[0])
		cmd.ErrCheck(err)
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.PushTimeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		events := make(chan local.Event)
		defer close(events)
		go handleEvents(events)
		err = buck.AddRemoteCid(
			ctx,
			target,
			args[1],
			local.WithSelectMerge(getSelectMergeStrategy(yes)),
			local.WithAddEvents(events),
		)
		cmd.ErrCheck(err)
		cmd.Success("Merged %s with %s", target, args[1])
	},
}

func getSelectMergeStrategy(auto bool) local.SelectMergeFunc {
	return func(desc string, isDir bool) (s local.MergeStrategy, err error) {
		if isDir {
			if auto {
				return local.Merge, nil
			}
			prompt := promptui.Select{
				Label: desc,
				Items: []local.MergeStrategy{local.Skip, local.Merge, local.Replace},
			}
			_, res, err := prompt.Run()
			if err != nil {
				return s, err
			}
			return local.MergeStrategy(res), nil
		} else {
			if auto {
				return local.Replace, nil
			}
			prompt := promptui.Prompt{
				Label:     desc,
				IsConfirm: true,
			}
			if _, err := prompt.Run(); err != nil {
				return local.Skip, nil
			}
			return local.Replace, nil
		}
	}
}
