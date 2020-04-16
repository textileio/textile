package main

import (
	"fmt"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(threadsCmd)
	orgsCmd.AddCommand(lsThreadsCmd)
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
	list, err := hub.ListThreads(ctx)
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
			data[i] = []string{id.String(), t.Name}
		}
		cmd.RenderTable([]string{"id", "name"}, data)
	}
	cmd.Message("Found %d threads", aurora.White(len(list.List)).Bold())
}

type threadItem struct {
	ID   string
	Name string
}

func selectThread(label, successMsg string) *threadItem {
	ctx, cancel := authCtx(cmdTimeout)
	defer cancel()
	list, err := hub.ListThreads(ctx)
	if err != nil {
		cmd.Fatal(err)
	}

	items := make([]*threadItem, len(list.List))
	for i, t := range list.List {
		id, err := thread.Cast(t.ID)
		if err != nil {
			cmd.Fatal(err)
		}
		items[i] = &threadItem{ID: id.String(), Name: t.Name}
	}
	items = append(items, &threadItem{ID: "Create new"})

	prompt := promptui.Select{
		Label: label,
		Items: items,
		Templates: &promptui.SelectTemplates{
			Active: fmt.Sprintf(`{{ "%s" | cyan }} {{ .ID | bold }} {{ .Name | faint | bold }}`,
				promptui.IconSelect),
			Inactive: `{{ .ID | faint }} {{ .Name | faint | bold }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		cmd.End("")
	}
	return items[index]
}
