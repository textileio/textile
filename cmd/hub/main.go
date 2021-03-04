package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/logrusorgru/aurora"
	mbase "github.com/multiformats/go-multibase"
	"github.com/spf13/cobra"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
	buck "github.com/textileio/textile/v2/cmd/buck/cli"
	hub "github.com/textileio/textile/v2/cmd/hub/cli"
)

var clients *cmd.Clients

func init() {
	cobra.OnInitialize(cmd.InitConfig(hub.Config()))
	hub.Init(rootCmd)
}

func main() {
	cmd.ErrCheck(rootCmd.Execute())
}

var rootCmd = &cobra.Command{
	Use:   hub.Name,
	Short: "Hub Client",
	Long:  `The Hub Client.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(hub.Config().Viper, hub.Config().Flags)

		hubTarget := hub.Config().Viper.GetString("api")
		minerIndexTarget := hub.Config().Viper.GetString("apiMinerIndex")
		clients = cmd.NewClients(hubTarget, true, minerIndexTarget)
		hub.SetClients(clients)

		config := local.DefaultConfConfig()
		config.EnvPrefix = fmt.Sprintf("%s_%s", strings.ToUpper(hub.Name), strings.ToUpper(buck.Name))
		buck.SetBucks(local.NewBucketsWithAuth(clients, config, hub.Auth))

		newIdentity, err := c.Flags().GetBool("newIdentity")
		cmd.ErrCheck(err)
		if newIdentity {
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
			hub.Config().Viper.Set("identity", sks)
			hub.Config().Viper.Set("token", "")
			cmd.WriteConfigToHome(hub.Config())
			cmd.Message("Generated new identity with public key %s", aurora.White(pks).Bold())
		}

		if hub.Config().Viper.GetString("identity") != "" &&
			hub.Config().Viper.GetString("token") == "" {
			_, id, err := mbase.Decode(hub.Config().Viper.GetString("identity"))
			if err != nil {
				cmd.Fatal(fmt.Errorf("invalid identity: %w", err))
			}
			sk, err := crypto.UnmarshalPrivateKey(id)
			cmd.ErrCheck(err)
			pkb, err := crypto.MarshalPublicKey(sk.GetPublic())
			cmd.ErrCheck(err)
			pks, err := mbase.Encode(mbase.Base32, pkb)
			cmd.ErrCheck(err)
			ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
			defer cancel()
			tok, err := clients.Threads.GetToken(hub.Auth(ctx), thread.NewLibp2pIdentity(sk))
			if err != nil {
				cmd.Fatal(fmt.Errorf("getting identity token: %w", err))
			}
			hub.Config().Viper.Set("token", string(tok))
			cmd.WriteConfigToHome(hub.Config())
			cmd.Message("Fetched new identity token for public key %s", aurora.White(pks).Bold())
		}

		if hub.Config().Viper.GetString("session") == "" &&
			hub.Config().Viper.GetString("apiKey") == "" &&
			c.Use != "init" &&
			c.Use != "login" &&
			c.Use != "version" &&
			c.Use != "update" &&
			c.Parent().Use != "index" {
			msg := "unauthorized! run `%s` or use `%s` to authorize"
			cmd.Fatal(errors.New(msg), aurora.Cyan(hub.Name+" init|login"), aurora.Cyan("--session"))
		}
	},
	PersistentPostRun: func(c *cobra.Command, args []string) {
		clients.Close()
	},
	Args: cobra.ExactArgs(0),
}
