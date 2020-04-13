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
	orgsCmd.AddCommand(createKeysCmd, invalidateKeysCmd, lsKeysCmd)
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
	Long:  `Create a new API key and secret. Keys are used by apps and services that leverage buckets and/or threads.`,
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		key, err := cloud.CreateKey(ctx)
		if err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("Created new API key and secret:\nKey: %s\nSecret: %s",
			aurora.White(key.Token).Bold(), aurora.White(key.Secret).Bold())
	},
}

var invalidateKeysCmd = &cobra.Command{
	Use:   "invalidate",
	Short: "Invalidate a key",
	Long:  `Invalidate a key. Invalidated keys cannot be used to create new threads.`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectKey("Invalidate key", aurora.Sprintf(
			aurora.BrightBlack("> Removing key {{ .Token | white | bold }}")))

		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		if err := cloud.InvalidateKey(ctx, selected.Token); err != nil {
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
	list, err := cloud.ListKeys(ctx)
	if err != nil {
		cmd.Fatal(err)
	}
	if len(list.List) > 0 {
		data := make([][]string, len(list.List))
		for i, k := range list.List {
			data[i] = []string{k.Token, strconv.FormatBool(k.Valid), strconv.Itoa(int(k.ThreadCount))}
		}
		cmd.RenderTable([]string{"token", "valid", "thread count"}, data)
	}
	cmd.Message("Found %d keys", aurora.White(len(list.List)).Bold())
}

type keyItem struct {
	Token       string
	ThreadCount int
}

func selectKey(label, successMsg string) *keyItem {
	ctx, cancel := authCtx(cmdTimeout)
	defer cancel()
	list, err := cloud.ListKeys(ctx)
	if err != nil {
		cmd.Fatal(err)
	}

	items := make([]*keyItem, len(list.List))
	for i, k := range list.List {
		if k.Valid {
			items[i] = &keyItem{Token: k.Token, ThreadCount: int(k.ThreadCount)}
		}
	}
	if len(items) == 0 {
		cmd.End("You don't have any keys!")
	}

	prompt := promptui.Select{
		Label: label,
		Items: items,
		Templates: &promptui.SelectTemplates{
			Active: fmt.Sprintf(`{{ "%s" | cyan }} {{ .Token | bold }} {{ .ThreadCount | faint | bold }}`,
				promptui.IconSelect),
			Inactive: `{{ .Token | faint }} {{ .ThreadCount | faint | bold }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		cmd.End("")
	}
	return items[index]
}
