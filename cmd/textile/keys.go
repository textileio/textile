package main

import (
	"fmt"
	"strconv"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(keysCmd)
	keysCmd.AddCommand(createKeysCmd, invalidateKeysCmd, lsKeysCmd)
}

var keysCmd = &cobra.Command{
	Use: "keys",
	Aliases: []string{
		"key",
	},
	Short: "Key management",
	Long:  `Manage your keys.`,
	Run: func(c *cobra.Command, args []string) {
		lsKeys()
	},
}

var createKeysCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an API key and secret",
	Long: `Create a new API key and secret. Keys are used by apps and services that leverage buckets and/or threads.

API secrets should be kept safely on a backend server, not in publicly readable client code.  
`,
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		k, err := hub.CreateKey(ctx)
		if err != nil {
			cmd.Fatal(err)
		}

		cmd.RenderTable([]string{"token", "secret"}, [][]string{{k.Token, k.Secret}})
		cmd.Success("Created new API key and secret")
	},
}

var invalidateKeysCmd = &cobra.Command{
	Use:   "invalidate",
	Short: "Invalidate a key",
	Long:  `Invalidate a key. Invalidated keys cannot be used to create new threads.`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectKey("Invalidate key", aurora.Sprintf(
			aurora.BrightBlack("> Invalidating key {{ .Token | white | bold }}")))

		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		if err := hub.InvalidateKey(ctx, selected.Token); err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("Invalidated key %s", aurora.White(selected.Token).Bold())
	},
}

var lsKeysCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List your keys",
	Long:  `List all of your keys.`,
	Run: func(c *cobra.Command, args []string) {
		lsKeys()
	},
}

func lsKeys() {
	ctx, cancel := authCtx(cmdTimeout)
	defer cancel()
	list, err := hub.ListKeys(ctx)
	if err != nil {
		cmd.Fatal(err)
	}
	if len(list.List) > 0 {
		data := make([][]string, len(list.List))
		for i, k := range list.List {
			data[i] = []string{k.Token, k.Secret, strconv.FormatBool(k.Valid), strconv.Itoa(int(k.Threads))}
		}
		cmd.RenderTable([]string{"token", "secret", "valid", "threads"}, data)
	}
	cmd.Message("Found %d keys", aurora.White(len(list.List)).Bold())
}

type keyItem struct {
	Token   string
	Threads int
}

func selectKey(label, successMsg string) *keyItem {
	ctx, cancel := authCtx(cmdTimeout)
	defer cancel()
	list, err := hub.ListKeys(ctx)
	if err != nil {
		cmd.Fatal(err)
	}

	items := make([]*keyItem, 0)
	for _, k := range list.List {
		if k.Valid {
			items = append(items, &keyItem{Token: k.Token, Threads: int(k.Threads)})
		}
	}
	if len(items) == 0 {
		cmd.End("You don't have any valid keys!")
	}

	prompt := promptui.Select{
		Label: label,
		Items: items,
		Templates: &promptui.SelectTemplates{
			Active: fmt.Sprintf(`{{ "%s" | cyan }} {{ .Token | bold }} {{ .Threads | faint | bold }}`,
				promptui.IconSelect),
			Inactive: `{{ .Token | faint }} {{ .Threads | faint | bold }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		cmd.End("")
	}
	return items[index]
}
