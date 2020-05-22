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
	orgsCmd.AddCommand(orgsCreateCmd, orgsLsCmd, orgsMembersCmd, orgsInviteCmd, orgsLeaveCmd, orgsDestroyCmd)
}

var orgsCmd = &cobra.Command{
	Use: "orgs",
	Aliases: []string{
		"org",
	},
	Short: "Org management",
	Long:  `Manages your organizations.`,
	Args:  cobra.ExactArgs(0),
}

var orgsCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create an org",
	Long:  `Creates a new organization.`,
	Args:  cobra.ExactArgs(0),
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
		_, err = hub.IsOrgNameAvailable(ctx, name)
		if err != nil {
			cmd.Fatal(err)
		}
		//url := fmt.Sprintf("%s/%s", res.Host, res.Slug)

		cmd.Message("The name of your Hub account will be %s", aurora.White(name).Bold())
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
		//url = fmt.Sprintf("%s/%s", org.Host, org.Slug)
		// @todo: Uncomment when dashboard's are live
		//cmd.Success("Created new org %s with URL %s", aurora.White(org.Name).Bold(), aurora.Underline(url))
		cmd.Success("Created new org %s", aurora.White(org.Name).Bold())
	},
}

var orgsLsCmd = &cobra.Command{
	Use: "ls",
	Aliases: []string{
		"list",
	},
	Short: "List orgs you're a member of",
	Long:  `Lists all the organizations that you're a member of.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
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
	},
}

var orgsMembersCmd = &cobra.Command{
	Use:   "members",
	Short: "List org members",
	Long:  `Lists current organization members.`,
	Args:  cobra.ExactArgs(0),
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

var orgsInviteCmd = &cobra.Command{
	Use:   "invite",
	Short: "Invite members to an org",
	Long:  `Invites a new member to an organization.`,
	Args:  cobra.ExactArgs(0),
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

var orgsLeaveCmd = &cobra.Command{
	Use:   "leave",
	Short: "Leave an org",
	Long:  `Leaves an organization.`,
	Args:  cobra.ExactArgs(0),
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

var orgsDestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy an org",
	Long:  `Destroys an organization and all associated data. You must be the org owner.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		selected := selectOrg("Destroy org", aurora.Sprintf(
			aurora.BrightBlack("> Destroying org {{ .Name | white | bold }}")))
		configViper.Set("org", selected.Slug)

		cmd.Warn("%s", aurora.Red("Are you absolutely sure? This action cannot be undone. The org and all associated data will be permanently deleted."))
		prompt := promptui.Prompt{
			Label: fmt.Sprintf("Please type '%s' to confirm", selected.Name),
			Validate: func(s string) error {
				if s != selected.Name {
					return fmt.Errorf("")
				}
				return nil
			},
		}
		if _, err := prompt.Run(); err != nil {
			cmd.End("")
		}

		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		if err := hub.RemoveOrg(ctx); err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("Org %s has been deleted", aurora.White(selected.Name).Bold())
	},
}
