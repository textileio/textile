package main

import (
	"fmt"
	"strconv"

	"github.com/manifoldco/promptui"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(threadsCmd)
	orgsCmd.AddCommand(lsThreadsCmd, useThreadsCmd)
}

var threadsCmd = &cobra.Command{
	Use: "threads",
	Aliases: []string{
		"thread",
	},
	Short: "Thread management",
	Long:  `Manage your threads.`,
	Run: func(c *cobra.Command, args []string) {
		lsThreads()
	},
}

var lsThreadsCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List your threads",
	Long:  `List all of your threads.`,
	Run: func(c *cobra.Command, args []string) {
		lsThreads()
	},
}

func lsThreads() {
	ctx, cancel := authCtx(cmdTimeout)
	defer cancel()
	list, err := cloud.ListThreads(ctx)
	if err != nil {
		cmd.Fatal(err)
	}
	if len(list.List) > 0 {
		data := make([][]string, len(list.List))
		for i, t := range list.List {
			id, err := thread.Cast(t.ID)
			if err != nil {
				cmd.Fatal(err)
			}
			data[i] = []string{id.String(), strconv.FormatBool(t.Primary)}
		}
		cmd.RenderTable([]string{"id", "primary"}, data)
	}
	cmd.Message("Found %d threads", aurora.White(len(list.List)).Bold())
}

var useThreadsCmd = &cobra.Command{
	Use:   "use",
	Short: "Select a thread as primary",
	Long:  `Use selects a thread as primary. The primary thread is used for new buckets.`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectThread("Select thread", aurora.Sprintf(
			aurora.BrightBlack("> Selected thread {{ .ID | white | bold }}")))

		id, err := thread.Decode(selected.ID)
		if err != nil {
			cmd.Fatal(err)
		}

		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		if err = cloud.UseThread(ctx, id); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Switched to thread %s", aurora.White(selected.ID).Bold())
	},
}

type threadItem struct {
	ID      string
	Primary string
}

func selectThread(label, successMsg string) *threadItem {
	ctx, cancel := authCtx(cmdTimeout)
	defer cancel()
	list, err := cloud.ListThreads(ctx)
	if err != nil {
		cmd.Fatal(err)
	}

	items := make([]*threadItem, len(list.List))
	for i, t := range list.List {
		id, err := thread.Cast(t.ID)
		if err != nil {
			cmd.Fatal(err)
		}
		items[i] = &threadItem{ID: id.String(), Primary: strconv.FormatBool(t.Primary)}
	}
	if len(items) == 0 {
		cmd.End("You don't have any threads!")
	}

	prompt := promptui.Select{
		Label: label,
		Items: items,
		Templates: &promptui.SelectTemplates{
			Active: fmt.Sprintf(`{{ "%s" | cyan }} {{ .ID | bold }} {{ .Primary | faint | bold }}`,
				promptui.IconSelect),
			Inactive: `{{ .ID | faint }} {{ .Primary | faint | bold }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		cmd.End("")
	}
	return items[index]
}
