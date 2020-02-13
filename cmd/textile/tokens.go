package main

import (
	"context"
	"fmt"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/cmd"
)

func init() {
	projectsCmd.AddCommand(tokensCmd)
	tokensCmd.AddCommand(addTokenCmd, lsTokensCmd, rmTokensCmd)
}

var tokensCmd = &cobra.Command{
	Use: "tokens",
	Aliases: []string{
		"token",
	},
	Short: "Project token management",
	Long:  `Manage your project's tokens.`,
	Run: func(c *cobra.Command, args []string) {
		lsTokens()
	},
}

var addTokenCmd = &cobra.Command{
	Use:   "add",
	Short: "Add project token",
	Long:  `Add a new project token (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		project := selectProject("Select project", aurora.Sprintf(
			aurora.BrightBlack("> Selected {{ .Name | white | bold }}")))

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		token, err := client.AddToken(
			ctx,
			project.Name,
			api.Auth{
				Token: authViper.GetString("token"),
			})
		if err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Added new token %s", aurora.White(token.ID).Bold())
	},
}

var lsTokensCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List project tokens",
	Long:  `List project tokens.`,
	Run: func(c *cobra.Command, args []string) {
		lsTokens()
	},
}

func lsTokens() {
	project := selectProject("Select project", aurora.Sprintf(
		aurora.BrightBlack("> Selected {{ .Name | white | bold }}")))

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	tokens, err := client.ListTokens(
		ctx,
		project.Name,
		api.Auth{
			Token: authViper.GetString("token"),
		})
	if err != nil {
		cmd.Fatal(err)
	}

	if len(tokens.List) > 0 {
		data := make([][]string, len(tokens.List))
		for i, t := range tokens.List {
			data[i] = []string{t}
		}
		cmd.RenderTable([]string{"id"}, data)
	}

	cmd.Message("Found %d tokens", aurora.White(len(tokens.List)).Bold())
}

var rmTokensCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Remove a token",
	Long:  `Remove a project token (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		project := selectProject("Select project", aurora.Sprintf(
			aurora.BrightBlack("> Selected {{ .Name | white | bold }}")))

		selected := selectToken("Remove token", aurora.Sprintf(
			aurora.BrightBlack("> Removing token {{ . | white | bold }}")),
			project.Name)

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.RemoveToken(
			ctx,
			selected,
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Removed token %s", aurora.White(selected).Bold())
	},
}

func selectToken(label, successMsg, project string) string {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	tokens, err := client.ListTokens(
		ctx,
		project,
		api.Auth{
			Token: authViper.GetString("token"),
		})
	if err != nil {
		cmd.Fatal(err)
	}

	if len(tokens.List) == 0 {
		cmd.End("You don't have any tokens!")
	}

	prompt := promptui.Select{
		Label: label,
		Items: tokens.List,
		Templates: &promptui.SelectTemplates{
			Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ . | bold }}`, promptui.IconSelect),
			Inactive: `{{ . | faint }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		cmd.End("")
	}

	return tokens.List[index]
}
