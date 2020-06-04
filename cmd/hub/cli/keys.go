package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	pb "github.com/textileio/textile/api/hub/pb"
	"github.com/textileio/textile/cmd"
	buck "github.com/textileio/textile/cmd/buck/cli"
)

var keysCmd = &cobra.Command{
	Use: "keys",
	Aliases: []string{
		"key",
	},
	Short: "API key management",
	Long:  `Manages your API keys.`,
	Args:  cobra.ExactArgs(0),
}

var keysCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an API key and secret",
	Long: `Creates a new API key and secret. Keys are used by apps and services that leverage buckets or threads.

Using the '--org' flag will create a new key under the Organization's account.

There are two types of API keys:
1. 'Account' keys provide direct access to developer/org account buckets and threads.
2. 'User Group' keys provide existing non-admin identities (e.g. app users) access to their own buckets and threads, using the resources of the parent account (i.e. the developer or organization). With this key type, you may specify multiple allowed origin domains from which the key signature is not required (useful for web applications), e.g., 'example.com, sub.example.com'.    

API secrets should be kept safely on a backend server, not in publicly readable client code.
`,
	Args: cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		org, err := c.Flags().GetString("org")
		if err != nil {
			cmd.Fatal(err)
		}
		if org != "" {
			buck.Config().Viper.Set("org", org)
		}

		prompt := promptui.Select{
			Label: "Select API key type",
			Items: []string{"account", "user group"},
			Templates: &promptui.SelectTemplates{
				Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ . | bold }}`, promptui.IconSelect),
				Inactive: `{{ . | faint }}`,
			},
		}
		index, keyTypeDesc, err := prompt.Run()
		if err != nil {
			cmd.End("")
		}

		var domains []string
		keyType := pb.KeyType(index)
		if keyType == pb.KeyType_USER {
			prompt := promptui.Prompt{
				Label: "Enter a comma-seperated list of allowed origin domains (optional)",
			}
			list, err := prompt.Run()
			if err != nil {
				cmd.End("")
			}
			domains = strings.Split(list, ",")
		}

		ctx, cancel := clients.Ctx.Auth(cmd.Timeout)
		defer cancel()
		k, err := clients.Hub.CreateKey(ctx, pb.KeyType(index), domains)
		if err != nil {
			cmd.Fatal(err)
		}

		dlist := strings.Join(k.Domains, ", ")
		cmd.RenderTable([]string{"key", "secret", "type", "domains"}, [][]string{{k.Key, k.Secret, keyTypeDesc, dlist}})
		cmd.Success("Created new API key and secret")
	},
}

var keysInvalidateCmd = &cobra.Command{
	Use:   "invalidate",
	Short: "Invalidate an API key",
	Long:  `Invalidates an API key. Invalidated keys cannot be used to create new threads.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		org, err := c.Flags().GetString("org")
		if err != nil {
			cmd.Fatal(err)
		}
		if org != "" {
			buck.Config().Viper.Set("org", org)
		}

		selected := selectKey("Invalidate key", aurora.Sprintf(
			aurora.BrightBlack("> Invalidating key {{ .Key | white | bold }}")))

		ctx, cancel := clients.Ctx.Auth(cmd.Timeout)
		defer cancel()
		if err := clients.Hub.InvalidateKey(ctx, selected.Key); err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("Invalidated key %s", aurora.White(selected.Key).Bold())
	},
}

var keysLsCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List your API keys",
	Long:  `Lists all of your API keys.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		org, err := c.Flags().GetString("org")
		if err != nil {
			cmd.Fatal(err)
		}
		if org != "" {
			buck.Config().Viper.Set("org", org)
		}

		ctx, cancel := clients.Ctx.Auth(cmd.Timeout)
		defer cancel()
		list, err := clients.Hub.ListKeys(ctx)
		if err != nil {
			cmd.Fatal(err)
		}
		if len(list.List) > 0 {
			data := make([][]string, len(list.List))
			for i, k := range list.List {
				domains := strings.Join(k.Domains, ", ")
				data[i] = []string{k.Key, k.Secret, keyTypeToString(k.Type), domains, strconv.FormatBool(k.Valid), strconv.Itoa(int(k.Threads))}
			}
			cmd.RenderTable([]string{"key", "secret", "type", "domains", "valid", "threads"}, data)
		}
		cmd.Message("Found %d keys", aurora.White(len(list.List)).Bold())
	},
}

type keyItem struct {
	Key     string
	Type    string
	Threads int
}

func selectKey(label, successMsg string) *keyItem {
	ctx, cancel := clients.Ctx.Auth(cmd.Timeout)
	defer cancel()
	list, err := clients.Hub.ListKeys(ctx)
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
		return "user group"
	}
	return
}
