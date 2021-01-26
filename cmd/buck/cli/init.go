package cli

import (
	"context"
	"errors"
	"fmt"

	cid "github.com/ipfs/go-cid"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/textile/v2/api/common"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new or existing bucket",
	Long: `Initializes a new or existing bucket.

A .textile config directory and a seed file will be created in the current working directory.
Existing configs will not be overwritten.

Use the '--existing' flag to interactively select an existing remote bucket.
Use the '--cid' flag to initialize from an existing UnixFS DAG.
Use the '--unfreeze' flag to retrieve '--cid' from known or imported deals.

By default, if the remote bucket exists, remote objects are pulled and merged with local changes.
Use the '--soft' flag to accept all local changes, including deletions.
Use the '--hard' flag to discard all local changes.
`,
	Args: cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)

		quiet, err := c.Flags().GetBool("quiet")
		cmd.ErrCheck(err)

		existing := conf.Thread.Defined() && conf.Key != ""
		chooseExisting, err := c.Flags().GetBool("existing")
		cmd.ErrCheck(err)
		if existing && chooseExisting {
			chooseExisting = false // Nothing left to choose
		}

		var strategy local.InitStrategy
		soft, err := c.Flags().GetBool("soft")
		cmd.ErrCheck(err)
		hard, err := c.Flags().GetBool("hard")
		cmd.ErrCheck(err)
		if soft && hard {
			cmd.Fatal(errors.New("--soft and --hard cannot by used together"))
		}
		if soft {
			strategy = local.Soft
		} else if hard {
			strategy = local.Hard
		} else {
			strategy = local.Hybrid
		}

		var xcid cid.Cid
		xcids, err := c.Flags().GetString("cid")
		cmd.ErrCheck(err)
		if xcids != "" {
			xcid, err = cid.Decode(xcids)
			cmd.ErrCheck(err)
		}
		if (existing || chooseExisting) && xcid.Defined() {
			cmd.Fatal(errors.New("--cid cannot be used with an existing bucket"))
		}

		// (jsign): re-enable when this feature is usable in mainnet.
		//unfreeze, err := c.Flags().GetBool("unfreeze")
		//cmd.ErrCheck(err)
		var unfreeze bool
		if unfreeze && xcid == cid.Undef {
			cmd.Fatal(errors.New("--unfreeze requires specifying --cid"))
		}

		var name string
		var private bool
		if !existing && !chooseExisting {
			if c.Flags().Changed("name") {
				name, err = c.Flags().GetString("name")
				cmd.ErrCheck(err)
			} else {
				namep := promptui.Prompt{
					Label: "Enter a name for your new bucket (optional)",
				}
				name, err = namep.Run()
				if err != nil {
					cmd.End("")
				}
			}
			if c.Flags().Changed("private") {
				private, err = c.Flags().GetBool("private")
				cmd.ErrCheck(err)
			} else {
				privp := promptui.Prompt{
					Label:     "Encrypt bucket contents",
					IsConfirm: true,
				}
				if _, err = privp.Run(); err == nil {
					private = true
				}
			}
		}

		if chooseExisting {
			ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
			defer cancel()
			list, err := bucks.RemoteBuckets(ctx, conf.Thread)
			cmd.ErrCheck(err)
			if len(list) == 0 {
				cmd.Fatal(fmt.Errorf("no existing buckets found"))
			}
			prompt := promptui.Select{
				Label: "Which existing bucket do you want to init from?",
				Items: list,
				Templates: &promptui.SelectTemplates{
					Active: fmt.Sprintf(`{{ "%s" | cyan }} {{ .Name | bold }} {{ .Key | faint | bold }}`,
						promptui.IconSelect),
					Inactive: `{{ .Name | faint }} {{ .Key | faint | bold }}`,
					Selected: aurora.Sprintf(aurora.BrightBlack("> Selected bucket {{ .Name | white | bold }}")),
				},
			}
			index, _, err := prompt.Run()
			if err != nil {
				cmd.End("")
			}
			selected := list[index]
			name = selected.Name
			conf.Thread = selected.Thread
			conf.Key = selected.Key
			existing = true
		}

		if !conf.Thread.Defined() {
			ctx, cancel := context.WithTimeout(bucks.Context(context.Background()), cmd.Timeout)
			defer cancel()
			selected := bucks.Clients().SelectThread(
				ctx,
				"Buckets are written to a threadDB. Select or create a new one",
				aurora.Sprintf(aurora.BrightBlack("> Selected threadDB {{ .Label | white | bold }}")),
				true)
			if selected.Label == "Create new" {
				if selected.Name == "" {
					prompt := promptui.Prompt{
						Label: "Enter a name for your new threadDB (optional)",
					}
					selected.Name, err = prompt.Run()
					if err != nil {
						cmd.End("")
					}
				}
				ctx = common.NewThreadNameContext(ctx, selected.Name)
				conf.Thread = thread.NewIDV1(thread.Raw, 32)
				err = bucks.Clients().Threads.NewDB(ctx, conf.Thread, db.WithNewManagedName(selected.Name))
				cmd.ErrCheck(err)
			} else {
				conf.Thread = selected.ID
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()

		var events chan local.Event
		if !quiet {
			events = make(chan local.Event)
			defer close(events)
			go handleEvents(events)
		}
		buck, err := bucks.NewBucket(
			ctx,
			conf,
			local.WithName(name),
			local.WithPrivate(private),
			local.WithCid(xcid),
			local.WithUnfreeze(unfreeze),
			local.WithStrategy(strategy),
			local.WithInitEvents(events))
		cmd.ErrCheck(err)

		if unfreeze {
			cmd.Message("The retrieval-id is: %s", buck.RetrievalID())
			cmd.Message("The bucket will be automatically created if the Filecoin retrieval succeeds.")
			cmd.Message("Track progress using `hub retrievals [ls | logs]`.")
			return
		}

		links, err := buck.RemoteLinks(ctx, "")
		cmd.ErrCheck(err)
		printLinks(links, DefaultFormat)

		var msg string
		if !existing {
			msg = "Initialized %s as a new empty bucket"
			if xcid.Defined() {
				msg = "Initialized %s as a new bootstrapped bucket"
			}
		} else {
			msg = "Initialized %s from an existing bucket"
		}

		bp, err := buck.Path()
		cmd.ErrCheck(err)
		cmd.Success(msg, aurora.White(bp).Bold())
	},
}
