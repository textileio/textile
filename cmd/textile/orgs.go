package main

import (
	"fmt"
	"net/mail"

	mbase "github.com/multiformats/go-multibase"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(orgsCmd)
	orgsCmd.AddCommand(createOrgsCmd, lsOrgsCmd, membersOrgsCmd, rmOrgsCmd, inviteOrgsCmd, leaveOrgsCmd)
}

var orgsCmd = &cobra.Command{
	Use: "orgs",
	Aliases: []string{
		"org",
	},
	Short: "Org management",
	Long:  `Manage your Organizations.`,
	Run: func(c *cobra.Command, args []string) {
		lsOrgs()
	},
}

var createOrgsCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create an Org",
	Long:  `Create a new Organization (interactive).`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		if _, err := hub.CreateOrg(ctx, args[0]); err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("Created new org %s", aurora.White(args[0]).Bold())
	},
}

var lsOrgsCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List Orgs you're a member of",
	Long:  `List all the Organizations that you're a member of.`,
	Run: func(c *cobra.Command, args []string) {
		lsOrgs()
	},
}

func lsOrgs() {
	ctx, cancel := authCtx(cmdTimeout)
	defer cancel()
	orgs, err := hub.ListOrgs(ctx)
	if err != nil {
		cmd.Fatal(err)
	}
	if len(orgs.List) > 0 {
		data := make([][]string, len(orgs.List))
		for i, o := range orgs.List {
			key, err := mbase.Encode(mbase.Base32, o.Key)
			if err != nil {
				cmd.Fatal(err)
			}
			data[i] = []string{o.Name, key}
		}
		cmd.RenderTable([]string{"name", "key"}, data)
	}
	cmd.Message("Found %d orgs", aurora.White(len(orgs.List)).Bold())
}

var membersOrgsCmd = &cobra.Command{
	Use:   "members",
	Short: "List Org members",
	Long:  `List current Organization members (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectOrg("Select org", aurora.Sprintf(
			aurora.BrightBlack("> Selected org {{ .Name | white | bold }}")))
		configViper.Set("org", selected.Name)

		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()

		org, err := hub.GetOrg(ctx)
		if err != nil {
			cmd.Fatal(err)
		}

		if len(org.Members) > 0 {
			data := make([][]string, len(org.Members))
			for i, m := range org.Members {
				key, err := mbase.Encode(mbase.Base32, m.Key)
				if err != nil {
					cmd.Fatal(err)
				}
				data[i] = []string{m.Username, key}
			}
			cmd.RenderTable([]string{"username", "key"}, data)
		}
		cmd.Message("Found %d members", aurora.White(len(org.Members)).Bold())
	},
}

var rmOrgsCmd = &cobra.Command{
	Use: "rm",
	Aliases: []string{
		"remove",
	},
	Short: "Remove an Org",
	Long:  `Remove an Organization (interactive). You must be the Org owner.`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectOrg("Remove org", aurora.Sprintf(
			aurora.BrightBlack("> Removing org {{ .Name | white | bold }}")))
		configViper.Set("org", selected.Name)

		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		if err := hub.RemoveOrg(ctx); err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("Removed org %s", aurora.White(selected.Name).Bold())
	},
}

var inviteOrgsCmd = &cobra.Command{
	Use:   "invite",
	Short: "Invite members to an Org",
	Long:  `Invite a new member to an Organization.`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectOrg("Select org", aurora.Sprintf(
			aurora.BrightBlack("> Selected org {{ .Name | white | bold }}")))
		configViper.Set("org", selected.Name)

		prompt := promptui.Prompt{
			Label: "Enter email to invite",
			Validate: func(email string) error {
				_, err := mail.ParseAddress(email)
				return err
			},
		}
		email, err := prompt.Run()
		if err != nil {
			cmd.End("")
		}

		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		if _, err := hub.InviteToOrg(ctx, email); err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("We sent %s an invitation to the %s org", aurora.White(email).Bold(),
			aurora.White(selected.Name).Bold())
	},
}

var leaveOrgsCmd = &cobra.Command{
	Use:   "leave",
	Short: "Leave an Org",
	Long:  `Leave an Organization (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectOrg("Leave org", aurora.Sprintf(
			aurora.BrightBlack("> Leaving org {{ .Name | white | bold }}")))
		configViper.Set("org", selected.Name)

		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		if err := hub.LeaveOrg(ctx); err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("Left org %s", aurora.White(selected.Name).Bold())
	},
}

type orgItem struct {
	Name  string
	Key   string
	Extra string
}

func selectOrg(label, successMsg string) *orgItem {
	ctx, cancel := authCtx(cmdTimeout)
	defer cancel()
	orgs, err := hub.ListOrgs(ctx)
	if err != nil {
		cmd.Fatal(err)
	}

	items := make([]*orgItem, len(orgs.List))
	for i, o := range orgs.List {
		key, err := mbase.Encode(mbase.Base32, o.Key)
		if err != nil {
			cmd.Fatal(err)
		}
		items[i] = &orgItem{Name: o.Name, Key: key}
	}
	if len(items) == 0 {
		cmd.End("You're not a member of any orgs!")
	}

	prompt := promptui.Select{
		Label: label,
		Items: items,
		Templates: &promptui.SelectTemplates{
			Active: fmt.Sprintf(`{{ "%s" | cyan }} {{ .Name | bold }} {{ .Extra | faint | bold }}`,
				promptui.IconSelect),
			Inactive: `{{ .Name | faint }} {{ .Extra | faint | bold }}`,
			Details:  `{{ "(Key:" | faint }} {{ .Key | faint }}{{ ")" | faint }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		cmd.End("")
	}
	return items[index]
}
