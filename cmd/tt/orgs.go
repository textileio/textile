package main

import (
	"fmt"
	"net/mail"
	"strconv"

	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	mbase "github.com/multiformats/go-multibase"
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
	Long:  `Manage your organizations.`,
	Run: func(c *cobra.Command, args []string) {
		lsOrgs()
	},
}

var createOrgsCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an org",
	Long:  `Create a new organization (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		prompt1 := promptui.Prompt{
			Label: "Choose an org name",
			Templates: &promptui.PromptTemplates{
				Valid: fmt.Sprintf("%s {{ . | bold }}%s ", bold(promptui.IconInitial), bold(":")),
			},
		}
		name, err := prompt1.Run()
		if err != nil {
			cmd.End("")
		}
		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		res, err := hub.IsOrgNameAvailable(ctx, name)
		if err != nil {
			cmd.Fatal(err)
		}
		url := fmt.Sprintf("%s/%s", res.Host, res.Slug)

		cmd.Message("The name of your account on Textile will be %s", aurora.White(name).Bold())
		// @todo: Uncomment when dashboard's are live
		//cmd.Message("Your URL will be %s", aurora.White(url).Bold())

		prompt2 := promptui.Prompt{
			Label:     "Please confirm",
			IsConfirm: true,
		}
		if _, err = prompt2.Run(); err != nil {
			cmd.End("")
		}

		ctx, cancel = authCtx(cmdTimeout)
		defer cancel()
		org, err := hub.CreateOrg(ctx, name)
		if err != nil {
			cmd.Fatal(err)
		}
		url = fmt.Sprintf("%s/%s", org.Host, org.Slug)
		cmd.Success("Created new org %s with URL %s", aurora.White(org.Name).Bold(), aurora.Underline(url))
	},
}

var lsOrgsCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List orgs you're a member of",
	Long:  `List all the organizations that you're a member of.`,
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
			url := fmt.Sprintf("%s/%s", o.Host, o.Slug)
			data[i] = []string{o.Name, url, key, strconv.Itoa(len(o.Members))}
		}
		cmd.RenderTable([]string{"name", "url", "key", "members"}, data)
	}
	cmd.Message("Found %d orgs", aurora.White(len(orgs.List)).Bold())
}

var membersOrgsCmd = &cobra.Command{
	Use:   "members",
	Short: "List org members",
	Long:  `List current organization members (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectOrg("Select org", aurora.Sprintf(
			aurora.BrightBlack("> Selected org {{ .Name | white | bold }}")))
		configViper.Set("org", selected.Slug)

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
	Short: "Remove an org",
	Long:  `Remove an organization (interactive). You must be the org owner.`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectOrg("Remove org", aurora.Sprintf(
			aurora.BrightBlack("> Removing org {{ .Name | white | bold }}")))
		configViper.Set("org", selected.Slug)

		prompt := promptui.Prompt{
			Label:     fmt.Sprintf("Delete the entire org and all associated data?"),
			IsConfirm: true,
		}
		if _, err := prompt.Run(); err != nil {
			cmd.End("")
		}

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
	Short: "Invite members to an org",
	Long:  `Invite a new member to an organization.`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectOrg("Select org", aurora.Sprintf(
			aurora.BrightBlack("> Selected org {{ .Name | white | bold }}")))
		configViper.Set("org", selected.Slug)

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
	Short: "Leave an org",
	Long:  `Leave an organization (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		selected := selectOrg("Leave org", aurora.Sprintf(
			aurora.BrightBlack("> Leaving org {{ .Name | white | bold }}")))
		configViper.Set("org", selected.Slug)

		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		if err := hub.LeaveOrg(ctx); err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("Left org %s", aurora.White(selected.Name).Bold())
	},
}

type orgItem struct {
	Name    string
	Slug    string
	Members string
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
		items[i] = &orgItem{Name: o.Name, Slug: o.Slug, Members: strconv.Itoa(len(o.Members))}
	}
	if len(items) == 0 {
		cmd.End("You're not a member of any orgs!")
	}

	prompt := promptui.Select{
		Label: label,
		Items: items,
		Templates: &promptui.SelectTemplates{
			Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ .Name | bold }} {{ .Slug | faint | bold }}`, promptui.IconSelect),
			Inactive: `{{ .Name | faint }} {{ .Slug | faint | bold }}`,
			Details:  `{{ "(Members:" | faint }} {{ .Members | faint }}{{ ")" | faint }}`,
			Selected: successMsg,
		},
	}
	index, _, err := prompt.Run()
	if err != nil {
		cmd.End("")
	}
	return items[index]
}
