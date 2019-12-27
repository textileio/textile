package main

import (
	"context"
	"fmt"

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
		team, err := client.AddTeam(
			ctx,
			args[0],
			api.Auth{
				Token: authViper.GetString("token"),
			})
		if err != nil {
			log.Fatal(err)
		}
		configViper.Set("scope", team.ID)

		if err := configViper.WriteConfig(); err != nil {
			cmd.Fatal(err)
		}

		fmt.Println(fmt.Sprintf("> Created new team, '%s'", args[0]))
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
		log.Fatal(err)
	}

	fmt.Println(fmt.Sprintf("> Found %d teams\n", len(teams.List)))

	data := make([][]string, len(teams.List))
	for i, t := range teams.List {
		data[i] = []string{t.Name, t.ID}
	}
	cmd.RenderTable([]string{"name", "id"}, data)
}

var membersTeamsCmd = &cobra.Command{
	Use:   "members",
	Short: "Display team members",
	Long:  `Display a list of current team members.`,
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
			log.Fatal(err)
		}

		fmt.Println(fmt.Sprintf("> Found %d members\n", len(team.Members)))

		data := make([][]string, len(team.Members))
		for i, m := range team.Members {
			data[i] = []string{m.Email, m.ID}
		}
		cmd.RenderTable([]string{"email", "id"}, data)
	},
}

var rmTeamsCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Remove a team",
	Long:  `Removes a team by its unique identifier (ID). You must be the team owner.`,
	Args: func(c *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("requires an ID argument")
		}
		return nil
	},
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.RemoveTeam(
			ctx,
			args[0],
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			log.Fatal(err)
		}
		fmt.Println("> Success")
	},
}

// @todo: Make interactive
var inviteTeamsCmd = &cobra.Command{
	Use:   "invite",
	Short: "Invite members",
	Long:  `Invite a new member to a team.`,
	Args: func(c *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("requires an ID argument")
		}
		if len(args) < 2 {
			return fmt.Errorf("requires an email argument")
		}
		return nil
	},
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.InviteToTeam(
			ctx,
			args[0],
			args[1],
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			log.Fatal(err)
		}
		fmt.Println(fmt.Sprintf("> We sent an invitation to %s", args[1]))
	},
}

var leaveTeamsCmd = &cobra.Command{
	Use:   "leave",
	Short: "Leave a team",
	Long:  `Leaves a team by its unique identifier (ID).`,
	Args: func(c *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("requires an ID argument")
		}
		return nil
	},
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.LeaveTeam(
			ctx,
			args[0],
			api.Auth{
				Token: authViper.GetString("token"),
			}); err != nil {
			log.Fatal(err)
		}
		fmt.Println("> Success")
	},
}

// @todo: Make interactive
var switchTeamsCmd = &cobra.Command{
	Use:   "switch",
	Short: "Switch teams",
	Long:  `Switch to a different team.`,
	Args: func(c *cobra.Command, args []string) error {
		if len(args) < 1 {
			return fmt.Errorf("requires an ID argument")
		}
		return nil
	},
	Run: func(c *cobra.Command, args []string) {

	},
}
