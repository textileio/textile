package cli

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/logrusorgru/aurora"
	"github.com/mitchellh/go-homedir"
	mbase "github.com/multiformats/go-multibase"
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
	powCmd.AddCommand(powAddrsCmd, powBalanceCmd, powConnectednessCmd, powFindPeerCmd, powHealthCmd, powInfoCmd, powPeersCmd, powRetrievalsCmd, powShowAllCmd, powShowCmd, powStorageCmd)
	billingCmd.AddCommand(billingSetupCmd, billingPortalCmd, billingUsageCmd, billingUsersCmd)
	rootCmd.AddCommand(bucketCmd)
	buck.Init(bucketCmd)

	rootCmd.PersistentFlags().String("api", config.Flags["api"].DefValue.(string), "API target")
	rootCmd.PersistentFlags().StringP("session", "s", config.Flags["session"].DefValue.(string), "User session token")
	rootCmd.PersistentFlags().StringP("org", "o", config.Flags["org"].DefValue.(string), "Org username")
	rootCmd.PersistentFlags().String("key", config.Flags["key"].DefValue.(string), "User API key")
	rootCmd.PersistentFlags().String("secret", config.Flags["secret"].DefValue.(string), "User API secret")
	rootCmd.PersistentFlags().String("identity", config.Flags["identity"].DefValue.(string), "User identity")

	billingUsageCmd.Flags().StringP("user", "u", "", "User multibase encoded public key")

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
	if config.Viper.GetString("identity") != "" {
		if config.Viper.GetString("identity") == "create" {
			sk, pk, err := crypto.GenerateEd25519Key(rand.Reader)
			cmd.ErrCheck(err)
			skb, err := crypto.MarshalPrivateKey(sk)
			cmd.ErrCheck(err)
			pkb, err := crypto.MarshalPublicKey(pk)
			cmd.ErrCheck(err)
			sks, err := mbase.Encode(mbase.Base32, skb)
			cmd.ErrCheck(err)
			pks, err := mbase.Encode(mbase.Base32, pkb)
			cmd.ErrCheck(err)
			config.Viper.Set("identity", sks)
			writeConfig()
			cmd.Message("Generated new identity with public key %s", aurora.White(pks).Bold())
		}
		_, id, err := mbase.Decode(config.Viper.GetString("identity"))
		if err != nil {
			cmd.Fatal(fmt.Errorf("invalid identity: %w", err))
		}
		sk, err := crypto.UnmarshalPrivateKey(id)
		cmd.ErrCheck(err)
		tok, err := clients.Threads.GetToken(ctx, thread.NewLibp2pIdentity(sk))
		if err != nil {
			cmd.Fatal(fmt.Errorf("getting identity token: %w", err))
		}
		ctx = thread.NewTokenContext(ctx, tok)
	}
	ctx = common.NewSessionContext(ctx, config.Viper.GetString("session"))
	return common.NewOrgSlugContext(ctx, config.Viper.GetString("org"))
}

func writeConfig() {
	home, err := homedir.Dir()
	cmd.ErrCheck(err)
	dir := filepath.Join(home, config.Dir)
	err = os.MkdirAll(dir, os.ModePerm)
	cmd.ErrCheck(err)
	filename := filepath.Join(dir, config.Name+".yml")
	err = config.Viper.WriteConfigAs(filename)
	cmd.ErrCheck(err)
}
