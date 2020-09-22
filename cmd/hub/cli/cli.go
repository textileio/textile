package cli

import (
	"context"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/cmd"
	buck "github.com/textileio/textile/cmd/buck/cli"
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
		},
		EnvPre: strings.ToUpper(Name),
		Global: true,
	}

	clients *cmd.Clients

	confirmTimeout = time.Hour
)

func Init(rootCmd *cobra.Command) {
	config.Viper.SetConfigType("yaml")

	rootCmd.AddCommand(initCmd, loginCmd, logoutCmd, whoamiCmd, destroyCmd, updateCmd, versionCmd)
	rootCmd.AddCommand(orgsCmd, keysCmd, threadsCmd, powCmd)
	orgsCmd.AddCommand(orgsCreateCmd, orgsLsCmd, orgsMembersCmd, orgsInviteCmd, orgsLeaveCmd, orgsDestroyCmd)
	keysCmd.AddCommand(keysCreateCmd, keysInvalidateCmd, keysLsCmd)
	threadsCmd.AddCommand(threadsLsCmd)
	powCmd.AddCommand(powAddrsCmd, powBalanceCmd, powConnectednessCmd, powFindPeerCmd, powHealthCmd, powInfoCmd, powNewAddrCmd, powPeersCmd, powRetrievalsCmd, powShowAllCmd, powShowCmd, powStorageCmd)
	rootCmd.AddCommand(bucketCmd)
	buck.Init(bucketCmd)

	rootCmd.PersistentFlags().String(
		"api",
		config.Flags["api"].DefValue.(string),
		"API target")

	rootCmd.PersistentFlags().StringP(
		"session",
		"s",
		config.Flags["session"].DefValue.(string),
		"User session token")

	rootCmd.PersistentFlags().StringP(
		"org",
		"o",
		config.Flags["org"].DefValue.(string),
		"Org username")

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
	ctx = common.NewSessionContext(ctx, config.Viper.GetString("session"))
	return common.NewOrgSlugContext(ctx, config.Viper.GetString("org"))
}
