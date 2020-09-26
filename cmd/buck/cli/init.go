package cli

import (
	"context"
	"errors"
	"fmt"

	cid "github.com/ipfs/go-cid"
	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	"github.com/textileio/textile/v2/api/common"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
	"github.com/textileio/uiprogress"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new or existing bucket",
	Long: `Initializes a new or existing bucket.

A .textile config directory and a seed file will be created in the current working directory.
Existing configs will not be overwritten.

Use the '--existing' flag to initialize from an existing remote bucket.
Use the '--cid' flag to initialize from an existing UnixFS DAG.
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

		var xcid cid.Cid
		xcids, err := c.Flags().GetString("cid")
		cmd.ErrCheck(err)
		if xcids != "" {
			xcid, err = cid.Decode(xcids)
			cmd.ErrCheck(err)
		}
		if (existing || chooseExisting) && xcid.Defined() {
			cmd.Fatal(errors.New("--cid can not be used with an existing bucket"))
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
				Label: "Which exiting bucket do you want to init from?",
				Items: list,
				Templates: &promptui.SelectTemplates{
					Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .Name | bold }} {{ .Key | faint | bold }}`, promptui.IconSelect),
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

		var events chan local.PathEvent
		var progress *uiprogress.Progress
		if !quiet {
			events = make(chan local.PathEvent)
			defer close(events)
			progress = uiprogress.New()
			progress.Start()
			go handleProgressBars(progress, events)
		}
		buck, err := bucks.NewBucket(
			ctx,
			conf,
			local.WithName(name),
			local.WithPrivate(private),
			local.WithCid(xcid),
			local.WithExistingPathEvents(events))
		if progress != nil {
			progress.Stop()
		}
		cmd.ErrCheck(err)

		links, err := buck.RemoteLinks(ctx, "")
		cmd.ErrCheck(err)
		printLinks(links)

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
