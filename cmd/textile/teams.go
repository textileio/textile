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
		inviteTeamsCmd,
		membersTeamsCmd,
		leaveTeamsCmd,
		rmTeamsCmd,
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
	_, err := client.ListTeams(
		ctx,
		api.Auth{
			Token: authViper.GetString("token"),
			Scope: configViper.GetString("scope"),
		})
	if err != nil {
		log.Fatal(err)
	}
}

var inviteTeamsCmd = &cobra.Command{
	Use:   "invite",
	Short: "Invite members",
	Long:  `Invite a new member to a team.`,
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.InviteToTeam(
			ctx,
			args[0],
			args[1],
			api.Auth{
				Token: authViper.GetString("token"),
				Scope: configViper.GetString("scope"),
			}); err != nil {
			log.Fatal(err)
		}
	},
}

var membersTeamsCmd = &cobra.Command{
	Use:   "members",
	Short: "Display team members",
	Long:  `Display a list of current team members.`,
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		_, err := client.GetTeam(
			ctx,
			args[0],
			api.Auth{
				Token: authViper.GetString("token"),
				Scope: configViper.GetString("scope"),
			})
		if err != nil {
			log.Fatal(err)
		}
	},
}

var leaveTeamsCmd = &cobra.Command{
	Use:   "leave",
	Short: "Leave a team",
	Long:  `Leaves a team by its unique identifier (ID).`,
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.LeaveTeam(
			ctx,
			args[0],
			api.Auth{
				Token: authViper.GetString("token"),
				Scope: configViper.GetString("scope"),
			}); err != nil {
			log.Fatal(err)
		}
	},
}

var rmTeamsCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Remove a team",
	Long:  `Removes a team by its unique identifier (ID). You must be the team owner.`,
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		if err := client.RemoveTeam(
			ctx,
			args[0],
			api.Auth{
				Token: authViper.GetString("token"),
				Scope: configViper.GetString("scope"),
			}); err != nil {
			log.Fatal(err)
		}
	},
}

var switchTeamsCmd = &cobra.Command{
	Use:   "switch",
	Short: "Switch teams",
	Long:  `Switch to a different team.`,
	Run: func(c *cobra.Command, args []string) {

	},
}
