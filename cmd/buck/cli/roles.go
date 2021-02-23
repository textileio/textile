package cli

import (
	"context"
	"fmt"
	"github.com/minio/sha256-simd"
	"github.com/multiformats/go-multibase"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/buckets"
	"github.com/textileio/textile/v2/cmd"
)

var rolesCmd = &cobra.Command{
	Use: "roles",
	Aliases: []string{
		"role",
	},
	Short: "Object access role management",
	Long:  `Manages remote bucket object access roles.`,
	Args:  cobra.ExactArgs(0),
}

var rolesGrantCmd = &cobra.Command{
	Use:   "grant [identity|token] [path]",
	Short: "Grant remote object access roles",
	Long: `Grants remote object access roles to an identity.

Identity can be either:
- a multibase encoded public key
- a "*" value to set the access roles for all objects
- if it is none of the above format it would be treated as a token that can be used to accept and transfer 
the role access later.

Access roles:
"none": Revokes all access.
"reader": Grants read-only access.
"writer": Grants read and write access.
"admin": Grants read, write, delete and role editing access.
`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(c *cobra.Command, args []string) {
		roleStr, err := c.Flags().GetString("role")
		cmd.ErrCheck(err)
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.PullTimeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		if roleStr == "" {
			roles := []string{"None", "Reader", "Writer", "Admin"}
			prompt := promptui.Select{
				Label: "Select a role",
				Items: roles,
				Templates: &promptui.SelectTemplates{
					Active:   fmt.Sprintf(`{{ "%s" | cyan }} {{ . | bold }}`, promptui.IconSelect),
					Inactive: `{{ . | faint }}`,
					Selected: aurora.Sprintf(aurora.BrightBlack("> Selected role {{ . | white | bold }}")),
				},
			}
			index, _, err := prompt.Run()
			if err != nil {
				cmd.End("")
			}
			roleStr = roles[index]
		}
		role, err := buckets.NewRoleFromString(roleStr)
		if err != nil {
			cmd.Error(fmt.Errorf("access role must be one of: none, reader, writer, or admin"))
		}
		var pth string
		if len(args) > 1 {
			pth = args[1]
		}

		identityOrToken := args[0]
		if err := buckets.ValidateAccessRoleKey(identityOrToken); err != nil {
			// we will use to identity as raw token, so we compute the hash
			bytes := sha256.Sum256([]byte(identityOrToken))
			identityOrToken, err = multibase.Encode(multibase.Base32, bytes[:])
			cmd.ErrCheck(err)
		}

		res, err := buck.PushPathAccessRoles(ctx, pth, map[string]buckets.Role{identityOrToken: role})
		cmd.ErrCheck(err)
		var data [][]string
		if len(res) > 0 {
			for i, r := range res {
				data = append(data, []string{i, r.String()})
			}
		}
		if len(data) > 0 {
			cmd.RenderTable([]string{"identity", "role"}, data)
		}
		cmd.Success("Updated access roles for path %s", aurora.White(pth).Bold())
	},
}

var rolesAcceptCmd = &cobra.Command{
	Use:   "accept [identity] [path] [token]",
	Short: "Accept remote object access roles using token",
	Long: `Grants remote object access roles to an identity using token.

Identity must be a multibase encoded public key.
`,
	Args: cobra.ExactArgs(3),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.PullTimeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		identity := args[0]
		pth := args[1]
		token := args[2]
		res, err := buck.AcceptPathAccessRoles(ctx, pth, token, identity)
		cmd.ErrCheck(err)
		cmd.Success("Access successfully granted to identity. Role granted is %s", aurora.White(res.String()).Bold())
	},
}

var rolesLsCmd = &cobra.Command{
	Use: "ls [path]",
	Aliases: []string{
		"list",
	},
	Short: "List top-level or nested bucket object access roles",
	Long:  `Lists top-level or nested bucket object access roles.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.PullTimeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		var pth string
		if len(args) > 0 {
			pth = args[0]
		}
		res, err := buck.PullPathAccessRoles(ctx, pth)
		cmd.ErrCheck(err)
		var data [][]string
		if len(res) > 0 {
			for i, r := range res {
				data = append(data, []string{i, r.String()})
			}
		}
		if len(data) > 0 {
			cmd.RenderTable([]string{"identity", "role"}, data)
		}
		cmd.Message("Found %d access roles", aurora.White(len(data)).Bold())
	},
}

