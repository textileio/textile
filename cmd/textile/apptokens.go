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
	rootCmd.AddCommand(appTokensCmd)
	appTokensCmd.AddCommand(
		addAppTokensCmd,
		lsAppTokensCmd,
		rmAppTokensCmd)
}

var appTokensCmd = &cobra.Command{
	Use: "tokens",
	Aliases: []string{
		"token",
	},
	Short: "Application token management",
	Long:  `Manage your project's application tokens.`,
	Run: func(c *cobra.Command, args []string) {
		lsTokens()
	},
}

var addAppTokensCmd = &cobra.Command{
	Use:   "add",
	Short: "Add app token",
	Long:  `Add a new application token (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		project := selectProject("Select project", aurora.Sprintf(
			aurora.BrightBlack("> Selected {{ .Name | white | bold }}")))

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		token, err := client.AddAppToken(
			ctx,
			project.ID,
			api.Auth{
				Token: authViper.GetString("token"),
			})
		if err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Added new app token %s", aurora.White(token.ID).Bold())
	},
}

var lsAppTokensCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List app tokens",
	Long:  `List application tokens for a project.`,
	Run: func(c *cobra.Command, args []string) {
		lsTokens()
	},
}

func lsTokens() {
	project := selectProject("Select project", aurora.Sprintf(
		aurora.BrightBlack("> Selected {{ .Name | white | bold }}")))

	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	tokens, err := client.ListAppTokens(
		ctx,
		project.ID,
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

var rmAppTokensCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Remove an app token",
	Long:  `Remove an app token (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		project := selectProject("Select project", aurora.Sprintf(
			aurora.BrightBlack("> Selected {{ .Name | white | bold }}")))

		selected := selectToken("Remove app token", aurora.Sprintf(
			aurora.BrightBlack("> Removing token {{ . | white | bold }}")),
			project.ID)

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.RemoveAppToken(
			ctx,
			selected,
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Removed app token %s", aurora.White(selected).Bold())
	},
}

func selectToken(label, successMsg, projID string) string {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	tokens, err := client.ListAppTokens(
		ctx,
		projID,
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
		log.Fatal(err)
	}

	return tokens.List[index]
}
