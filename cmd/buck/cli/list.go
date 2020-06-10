package cli

import (
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	pb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/cmd"
)

var bucketLsCmd = &cobra.Command{
	Use: "ls [path]",
	Aliases: []string{
		"list",
	},
	Short: "List top-level or nested bucket objects",
	Long:  `Lists top-level or nested bucket objects.`,
	Args:  cobra.MaximumNArgs(1),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(config.Viper, config.Flags)
		if config.Viper.ConfigFileUsed() == "" {
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

		ctx, cancel := clients.Ctx.Thread(cmd.Timeout)
		defer cancel()
		key := config.Viper.GetString("key")
		rep, err := clients.Buckets.ListPath(ctx, key, pth)
		if err != nil {
			cmd.Fatal(err)
		}
		var items []*pb.ListPathItem
		if len(rep.Item.Items) > 0 {
			items = rep.Item.Items
		} else if !rep.Item.IsDir {
			items = append(items, rep.Item)
		}

		var data [][]string
		if len(items) > 0 && !strings.HasPrefix(pth, config.Dir) {
			for _, item := range items {
				if item.Name == config.Dir {
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
