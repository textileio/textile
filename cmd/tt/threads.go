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
	threadsCmd.AddCommand(threadsLsCmd)

	threadsCmd.PersistentFlags().String("org", "", "Org username")
}

var threadsCmd = &cobra.Command{
	Use: "threads",
	Aliases: []string{
		"thread",
	},
	Short: "Thread management",
	Long:  `Manages your threads.`,
	Args:  cobra.ExactArgs(0),
}

var threadsLsCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List your threads",
	Long:  `Lists all of your threads.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		org, err := c.Flags().GetString("org")
		if err != nil {
			cmd.Fatal(err)
		}
		if org != "" {
			configViper.Set("org", org)
		}

		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		list, err := users.ListThreads(ctx)
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
				if t.Name == "" {
					t.Name = "unnamed"
				}
				data[i] = []string{id.String(), t.Name, getThreadType(t.IsDB)}
			}
			cmd.RenderTable([]string{"id", "name", "type"}, data)
		}
		cmd.Message("Found %d threads", aurora.White(len(list.List)).Bold())
	},
}

type threadItem struct {
	ID   string
	Name string
	Type string
}

func selectThread(label, successMsg string, dbsOnly bool) *threadItem {
	ctx, cancel := authCtx(cmdTimeout)
	defer cancel()
	list, err := users.ListThreads(ctx)
	if err != nil {
		cmd.Fatal(err)
	}

	var items []*threadItem
	for _, t := range list.List {
		if dbsOnly && !t.IsDB {
			continue
		}
		id, err := thread.Cast(t.ID)
		if err != nil {
			cmd.Fatal(err)
		}
		if t.Name == "" {
			t.Name = "unnamed"
		}
		items = append(items, &threadItem{
			ID:   id.String(),
			Name: t.Name,
			Type: getThreadType(t.IsDB),
		})
	}
	var name string
	if len(items) == 0 {
		name = "default"
	}
	items = append(items, &threadItem{ID: "Create new", Name: name, Type: "db"})

	prompt := promptui.Select{
		Label: label,
		Items: items,
		Templates: &promptui.SelectTemplates{
			Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .ID | bold }} {{ .Name | faint | bold }}`, promptui.IconSelect),
			Inactive: `{{ .ID | faint }} {{ .Name | faint | bold }}`,
			Details:  `{{ "(Type:" | faint }} {{ .Type | faint }}{{ ")" | faint }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		cmd.End("")
	}
	return items[index]
}

func getThreadType(isDB bool) string {
	if isDB {
		return "db"
	} else {
		return "log"
	}
}
