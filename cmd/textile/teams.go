package main

import (
	"context"
	"errors"
	"fmt"
	"net/mail"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(teamsCmd)
	teamsCmd.AddCommand(
		addTeamsCmd,
		lsTeamsCmd,
		membersTeamsCmd,
		rmTeamsCmd,
		inviteTeamsCmd,
		leaveTeamsCmd,
		switchTeamsCmd)
}

var teamsCmd = &cobra.Command{
	Use: "teams",
	Aliases: []string{
		"team",
	},
	Short: "Team management",
	Long:  `Manage your teams.`,
	Run: func(c *cobra.Command, args []string) {
		lsTeams()
	},
}

var addTeamsCmd = &cobra.Command{
	Use:   "add",
	Short: "Add team",
	Long:  `Add a new team (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		prompt := promptui.Prompt{
			Label: "Enter a team name",
			Validate: func(name string) error {
				if len(name) < 3 {
					return errors.New("name too short")
				}
				return nil
			},
		}
		name, err := prompt.Run()
		if err != nil {
			log.Fatal(err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if _, err := client.AddTeam(
			ctx,
			name,
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Added new team %s", aurora.White(name).Bold())
	},
}

var lsTeamsCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List teams you're a member of",
	Long:  `List all the teams that you're a member of.`,
	Run: func(c *cobra.Command, args []string) {
		lsTeams()
	},
}

func lsTeams() {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	teams, err := client.ListTeams(
		ctx,
		api.Auth{
			Token: authViper.GetString("token"),
		})
	if err != nil {
		cmd.Fatal(err)
	}

	data := make([][]string, len(teams.List))
	for i, t := range teams.List {
		data[i] = []string{t.Name, t.ID}
	}
	cmd.RenderTable([]string{"name", "id"}, data)

	cmd.Message("Found %d teams", aurora.White(len(teams.List)).Bold())
}

var membersTeamsCmd = &cobra.Command{
	Use:   "members",
	Short: "List team members",
	Long:  `List current team members.`,
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		team, err := client.GetTeam(
			ctx,
			args[0],
			api.Auth{
				Token: authViper.GetString("token"),
			})
		if err != nil {
			cmd.Fatal(err)
		}

		data := make([][]string, len(team.Members))
		for i, m := range team.Members {
			data[i] = []string{m.Email, m.ID}
		}
		cmd.RenderTable([]string{"email", "id"}, data)

		cmd.Message("Found %d members", aurora.White(len(team.Members)).Bold())
	},
}

var rmTeamsCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Remove a team",
	Long:  `Remove a team (interactive). You must be the team owner.`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectTeam("Remove team", aurora.Sprintf(
			aurora.BrightBlack("> Removing team {{ .Name | white | bold }}")),
			false)

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.RemoveTeam(
			ctx,
			selected.ID,
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Removed team %s", aurora.White(selected.Name).Bold())
	},
}

var inviteTeamsCmd = &cobra.Command{
	Use:   "invite",
	Short: "Invite members",
	Long:  `Invite a new member to a team.`,
	Run: func(c *cobra.Command, args []string) {
		scope := configViper.GetString("scope")
		if scope == "" {
			msg := "please select a team scope using `%s` or use `%s`"
			cmd.Fatal(errors.New(msg),
				aurora.Cyan("textile switch"), aurora.Cyan("--scope"))
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		team, err := client.GetTeam(
			ctx,
			scope,
			api.Auth{
				Token: authViper.GetString("token"),
			})
		if err != nil {
			cmd.Fatal(err)
		}

		prompt := promptui.Prompt{
			Label: "Enter email to invite",
			Validate: func(email string) error {
				_, err := mail.ParseAddress(email)
				return err
			},
		}
		email, err := prompt.Run()
		if err != nil {
			log.Fatal(err)
		}

		ctx2, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if _, err := client.InviteToTeam(
			ctx2,
			team.ID,
			email,
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("We sent %s an invitation to %s", aurora.White(email).Bold(),
			aurora.White(team.Name).Bold())
	},
}

var leaveTeamsCmd = &cobra.Command{
	Use:   "leave",
	Short: "Leave a team",
	Long:  `Leave a team (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectTeam("Leave team", aurora.Sprintf(
			aurora.BrightBlack("> Leaving team {{ .Name | white | bold }}")),
			false)

		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.LeaveTeam(
			ctx,
			selected.ID,
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Left team %s", aurora.White(selected.Name).Bold())
	},
}

var switchTeamsCmd = &cobra.Command{
	Use:   "switch",
	Short: "Switch teams",
	Long:  `Switch to a different team.`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectTeam("Switch to team", aurora.Sprintf(aurora.Cyan("> Success! %s"),
			aurora.Sprintf(aurora.BrightBlack("Switched to team {{ .Name | white | bold }}"))),
			false)

		configViper.Set("scope", selected.ID)
		if err := configViper.WriteConfig(); err != nil {
			cmd.Fatal(err)
		}
	},
}

type teamItem struct {
	ID      string
	Name    string
	Display string
}

func selectTeam(label, successMsg string, includeAccount bool) *teamItem {
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	teams, err := client.ListTeams(
		ctx,
		api.Auth{
			Token: authViper.GetString("token"),
		})
	if err != nil {
		cmd.Fatal(err)
	}

	items := make([]teamItem, len(teams.List))
	for i, t := range teams.List {
		items[i] = teamItem{ID: t.ID, Name: t.Name, Display: t.Name}
	}

	if includeAccount {
		ctx2, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		who, err := client.Whoami(
			ctx2,
			api.Auth{
				Token: authViper.GetString("token"),
				Scope: configViper.GetString("scope"),
			})
		if err != nil {
			cmd.Fatal(err)
		}
		items = append([]teamItem{{
			ID:      who.ID,
			Name:    who.Email,
			Display: who.Email,
		}}, items...)

		for i, t := range items {
			fmt.Println(t.ID, who.TeamID)
			if t.ID == who.TeamID {
				items[i].Display += " (current)"
			}
		}
	}

	prompt := promptui.Select{
		Label: label,
		Items: items,
		Templates: &promptui.SelectTemplates{
			Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .Display | bold }}`, promptui.IconSelect),
			Inactive: `{{ .Display | faint }}`,
			Details:  `{{ "(ID:" | faint }} {{ .ID | faint }}{{ ")" | faint }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		log.Fatal(err)
	}

	return &items[index]
}
