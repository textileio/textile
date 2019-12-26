package main

import (
	"context"
	"fmt"

	"github.com/textileio/textile/cmd"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(teamsCmd)
	teamsCmd.AddCommand(addCmd)
}

var teamsCmd = &cobra.Command{
	Use:   "teams",
	Short: "Team management",
	Long:  `Manage your Textile teams.`,
}

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Add team",
	Long:  `Add a new team.`,
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
			authViper.GetString("token"))
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
