package cli

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/textileio/textile/cmd"
	buck "github.com/textileio/textile/cmd/buck/cli"
)

const Name = "hub"

var (
	config = cmd.Config{
		Viper: viper.New(),
		Dir:   ".textile",
		Name:  "auth",
		Flags: map[string]cmd.Flag{
			"api": {
				Key:      "api",
				DefValue: "api.textile.io:443",
			},
			"session": {
				Key:      "session",
				DefValue: "",
			},
		},
		EnvPre: "HUB",
		Global: true,
	}

	clients *cmd.Clients

	confirmTimeout = time.Hour
)

func Init(rootCmd *cobra.Command) {
	rootCmd.AddCommand(initCmd, loginCmd, logoutCmd, whoamiCmd, destroyCmd)
	rootCmd.AddCommand(orgsCmd, keysCmd, threadsCmd)
	orgsCmd.AddCommand(orgsCreateCmd, orgsLsCmd, orgsMembersCmd, orgsInviteCmd, orgsLeaveCmd, orgsDestroyCmd)
	keysCmd.AddCommand(keysCreateCmd, keysInvalidateCmd, keysLsCmd)
	threadsCmd.AddCommand(threadsLsCmd)
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

	if err := cmd.BindFlags(config.Viper, rootCmd, config.Flags); err != nil {
		cmd.Fatal(err)
	}

	keysCmd.PersistentFlags().String("org", "", "Org username")
	threadsCmd.PersistentFlags().String("org", "", "Org username")
}

func Config() cmd.Config {
	return config
}

func SetClients(c *cmd.Clients) {
	clients = c
}
