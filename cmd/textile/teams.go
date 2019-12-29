package main

import (
	"context"
	"fmt"
	"net/mail"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	api "github.com/textileio/textile/api/client"
	"github.com/textileio/textile/api/pb"
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
	Long:  `Add a new team with the given name.`,
	Args: func(c *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("requires a name argument")
		}
		return nil
	},
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if _, err := client.AddTeam(
			ctx,
			args[0],
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			cmd.Fatal(err)
		}

		cmd.Success("Added new team %s", aurora.White(args[0]).Bold())
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
		selected := selectTeam(aurora.Sprintf(
			aurora.BrightBlack("> Removing team {{ .Name | white | bold }}")))

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
			cmd.Fatal(fmt.Errorf(
				"please select a team scope using `textile switch` or use `--scope`"))
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
			cmd.Fatal(err)
			return
		}

		ctx2, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.InviteToTeam(
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
		selected := selectTeam(aurora.Sprintf(
			aurora.BrightBlack("> Leaving team {{ .Name | white | bold }}")))

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
		selected := selectTeam(aurora.Sprintf(aurora.Cyan("> Success! %s"),
			aurora.Sprintf(aurora.BrightBlack("Switched to team {{ .Name | white | bold }}"))))

		configViper.Set("scope", selected.ID)
		if err := configViper.WriteConfig(); err != nil {
			cmd.Fatal(err)
		}
	},
}

func selectTeam(successMsg string) *pb.GetTeamReply {
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

	prompt := promptui.Select{
		Label: "Switch to team",
		Items: teams.List,
		Templates: &promptui.SelectTemplates{
			Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .Name | bold }}`, promptui.IconSelect),
			Inactive: `{{ .Name | faint }}`,
			Details:  `{{ "(ID:" | faint }} {{ .ID | faint }}{{ ")" | faint }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		cmd.Fatal(err)
	}

	return teams.List[index]
}
