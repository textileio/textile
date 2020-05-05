package main

import (
	"fmt"
	"strconv"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	pb "github.com/textileio/textile/api/hub/pb"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(keysCmd)
	keysCmd.AddCommand(createKeysCmd, invalidateKeysCmd, lsKeysCmd)

	keysCmd.PersistentFlags().String("org", "", "Org username")
}

var keysCmd = &cobra.Command{
	Use: "keys",
	Aliases: []string{
		"key",
	},
	Short: "Key management",
	Long:  `Manage your keys.`,
	Run: func(c *cobra.Command, args []string) {
		lsKeys(c)
	},
}

var createKeysCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an API key and secret",
	Long: `Create a new API key and secret. Keys are used by apps and services that leverage buckets or threads.

Using the '--org' flag will create a new key under the Organization's account.

There are two types of API keys:
1. 'Account' keys provide direct access to developer/org account buckets and threads.
1. 'User' keys provide existing external identities (users) access to their own buckets and threads, under the custodianship of the parent account.  

API secrets should be kept safely on a backend server, not in publicly readable client code.
`,
	Run: func(c *cobra.Command, args []string) {
		org, err := c.Flags().GetString("org")
		if err != nil {
			cmd.Fatal(err)
		}
		if org != "" {
			configViper.Set("org", org)
		}

		prompt := promptui.Select{
			Label: "Select API key type",
			Items: []string{"account", "user"},
			Templates: &promptui.SelectTemplates{
				Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ . | bold }}`, promptui.IconSelect),
				Inactive: `{{ . | faint }}`,
			},
		}
		index, keyType, err := prompt.Run()
		if err != nil {
			cmd.End("")
		}

		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		k, err := hub.CreateKey(ctx, pb.KeyType(index))
		if err != nil {
			cmd.Fatal(err)
		}

		cmd.RenderTable([]string{"key", "secret", "type"}, [][]string{{k.Key, k.Secret, keyType}})
		cmd.Success("Created new API key and secret")
	},
}

var invalidateKeysCmd = &cobra.Command{
	Use:   "invalidate",
	Short: "Invalidate a key",
	Long:  `Invalidate a key. Invalidated keys cannot be used to create new threads.`,
	Run: func(c *cobra.Command, args []string) {
		org, err := c.Flags().GetString("org")
		if err != nil {
			cmd.Fatal(err)
		}
		if org != "" {
			configViper.Set("org", org)
		}

		selected := selectKey("Invalidate key", aurora.Sprintf(
			aurora.BrightBlack("> Invalidating key {{ .Key | white | bold }}")))

		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		if err := hub.InvalidateKey(ctx, selected.Key); err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("Invalidated key %s", aurora.White(selected.Key).Bold())
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
		lsKeys(c)
	},
}

func lsKeys(c *cobra.Command) {
	org, err := c.Flags().GetString("org")
	if err != nil {
		cmd.Fatal(err)
	}
	if org != "" {
		configViper.Set("org", org)
	}

	ctx, cancel := authCtx(cmdTimeout)
	defer cancel()
	list, err := hub.ListKeys(ctx)
	if err != nil {
		cmd.Fatal(err)
	}
	if len(list.List) > 0 {
		data := make([][]string, len(list.List))
		for i, k := range list.List {
			data[i] = []string{k.Key, k.Secret, keyTypeToString(k.Type), strconv.FormatBool(k.Valid), strconv.Itoa(int(k.Threads))}
		}
		cmd.RenderTable([]string{"key", "secret", "type", "valid", "threads"}, data)
	}
	cmd.Message("Found %d keys", aurora.White(len(list.List)).Bold())
}

type keyItem struct {
	Key     string
	Type    string
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
			items = append(items, &keyItem{Key: k.Key, Type: keyTypeToString(k.Type), Threads: int(k.Threads)})
		}
	}
	if len(items) == 0 {
		cmd.End("You don't have any valid keys!")
	}

	prompt := promptui.Select{
		Label: label,
		Items: items,
		Templates: &promptui.SelectTemplates{
			Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .Key | bold }} {{ .Type | faint }}`, promptui.IconSelect),
			Inactive: `{{ .Key | faint }} {{ .Type | faint }}`,
			Details:  `{{ "(Threads:" | faint }} {{ .Threads | faint }}{{ ")" | faint }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		cmd.End("")
	}
	return items[index]
}

func keyTypeToString(t pb.KeyType) (s string) {
	switch t {
	case pb.KeyType_ACCOUNT:
		return "account"
	case pb.KeyType_USER:
		return "user"
	}
	return
}
