package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/v2/api/common"
	"github.com/textileio/textile/v2/cmd"
	buck "github.com/textileio/textile/v2/cmd/buck/cli"
)

const Name = "hub"

var (
	config = &cmd.Config{
		Viper: viper.New(),
		Dir:   ".textile",
		Name:  "auth",
		Flags: map[string]cmd.Flag{
			"api": {
				Key:      "api",
				DefValue: "api.hub.textile.io:443",
			},
			"session": {
				Key:      "session",
				DefValue: "",
			},
			"org": {
				Key:      "org",
				DefValue: "",
			},
			"key": {
				Key:      "key",
				DefValue: "",
			},
			"secret": {
				Key:      "secret",
				DefValue: "",
			},
			"token": {
				Key:      "token",
				DefValue: "",
			},
			"identity": {
				Key:      "identity",
				DefValue: "",
			},
		},
		EnvPre: strings.ToUpper(Name),
		Global: true,
	}

	clients *cmd.Clients

	confirmTimeout = time.Hour
)

func Init(rootCmd *cobra.Command) {
	config.Viper.SetConfigType("yaml")

	rootCmd.AddCommand(initCmd, loginCmd, logoutCmd, whoamiCmd, destroyCmd, updateCmd, versionCmd, orgsCmd, keysCmd, threadsCmd, powCmd, billingCmd)
	orgsCmd.AddCommand(orgsCreateCmd, orgsLsCmd, orgsMembersCmd, orgsInviteCmd, orgsLeaveCmd, orgsDestroyCmd)
	keysCmd.AddCommand(keysCreateCmd, keysInvalidateCmd, keysLsCmd)
	threadsCmd.AddCommand(threadsLsCmd)
	powCmd.AddCommand(powAddrsCmd, powBalanceCmd, powInfoCmd, powStorageCmd, powRetrievalsCmd)
	billingCmd.AddCommand(billingSetupCmd, billingPortalCmd, billingUsageCmd, billingUsersCmd)
	rootCmd.AddCommand(bucketCmd)
	buck.Init(bucketCmd)

	rootCmd.PersistentFlags().String("api", config.Flags["api"].DefValue.(string), "API target")
	rootCmd.PersistentFlags().StringP("session", "s", config.Flags["session"].DefValue.(string), "User session token")
	rootCmd.PersistentFlags().StringP("org", "o", config.Flags["org"].DefValue.(string), "Org username")
	rootCmd.PersistentFlags().String("key", config.Flags["key"].DefValue.(string), "User API key")
	rootCmd.PersistentFlags().String("secret", config.Flags["secret"].DefValue.(string), "User API secret")
	rootCmd.PersistentFlags().String("token", config.Flags["token"].DefValue.(string), "User identity token")
	rootCmd.PersistentFlags().String("identity", config.Flags["identity"].DefValue.(string), "User identity")

	rootCmd.PersistentFlags().Bool("newIdentity", false, "Generate a new user identity")

	billingUsageCmd.Flags().StringP("user", "u", "", "User multibase encoded public key")

	billingUsersCmd.Flags().Int64("limit", 25, "Page size (max 1000)")
	billingUsersCmd.Flags().Int64("offset", 0, "Page offset (returned by each request)")

	err := cmd.BindFlags(config.Viper, rootCmd, config.Flags)
	cmd.ErrCheck(err)
}

func Config() *cmd.Config {
	return config
}

func SetClients(c *cmd.Clients) {
	clients = c
}

func Auth(ctx context.Context) context.Context {
	if config.Viper.GetString("key") != "" {
		ctx = common.NewAPIKeyContext(ctx, config.Viper.GetString("key"))
		if config.Viper.GetString("secret") != "" {
			var err error
			ctx, err = common.CreateAPISigContext(
				ctx,
				time.Now().Add(time.Hour),
				config.Viper.GetString("secret"),
			)
			if err != nil {
				cmd.Fatal(fmt.Errorf("invalid secret: %w", err))
			}
		}
		ctx = thread.NewTokenContext(ctx, thread.Token(config.Viper.GetString("token")))
	} else {
		ctx = common.NewSessionContext(ctx, config.Viper.GetString("session"))
		ctx = common.NewOrgSlugContext(ctx, config.Viper.GetString("org"))
	}
	return ctx
}
