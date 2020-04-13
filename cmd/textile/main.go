package main

import (
	"context"
	"crypto/tls"
	"errors"
	"strings"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	tc "github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/core/thread"
	bc "github.com/textileio/textile/api/buckets/client"
	cc "github.com/textileio/textile/api/cloud/client"
	"github.com/textileio/textile/api/common"
	"github.com/textileio/textile/cmd"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	authFile  string
	authViper = viper.New()

	authFlags = map[string]cmd.Flag{
		"api": {
			Key:      "api",
			DefValue: "api.textile.io:443",
		},
		"session": {
			Key:      "session",
			DefValue: "",
		},
	}

	configFile  string
	configViper = viper.New()

	flags = map[string]cmd.Flag{
		"name": {
			Key:      "name",
			DefValue: "",
		},
		"org": {
			Key:      "org",
			DefValue: "",
		},
		"public": {
			Key:      "public",
			DefValue: false,
		},
		"thread": {
			Key:      "thread",
			DefValue: "",
		},
	}

	cloud   *cc.Client
	buckets *bc.Client
	threads *tc.Client

	cmdTimeout     = time.Second * 10
	loginTimeout   = time.Minute * 3
	addFileTimeout = time.Hour * 24
	getFileTimeout = time.Hour * 24
)

func init() {
	rootCmd.AddCommand(whoamiCmd)

	cobra.OnInitialize(cmd.InitConfig(authViper, authFile, ".textile", "auth", true))
	cobra.OnInitialize(cmd.InitConfig(configViper, configFile, ".textile", "config", false))

	rootCmd.PersistentFlags().String(
		"api",
		authFlags["api"].DefValue.(string),
		"API target")

	rootCmd.PersistentFlags().StringP(
		"session",
		"s",
		authFlags["session"].DefValue.(string),
		"User session token")

	if err := cmd.BindFlags(authViper, rootCmd, authFlags); err != nil {
		cmd.Fatal(err)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		cmd.Fatal(err)
	}
}

var rootCmd = &cobra.Command{
	Use:   "textile",
	Short: "Textile client",
	Long:  `The Textile client.`,
	PersistentPreRun: func(c *cobra.Command, args []string) {
		authViper.SetConfigType("yaml")
		configViper.SetConfigType("yaml")

		cmd.ExpandConfigVars(authViper, authFlags)

		if authViper.GetString("session") == "" && c.Use != "login" {
			msg := "unauthorized! run `%s` or use `%s` to authorize"
			cmd.Fatal(errors.New(msg),
				aurora.Cyan("textile login"), aurora.Cyan("--session"))
		}

		var opts []grpc.DialOption
		auth := common.Credentials{}
		target := authViper.GetString("api")
		if strings.Contains(target, "443") {
			creds := credentials.NewTLS(&tls.Config{})
			opts = append(opts, grpc.WithTransportCredentials(creds))
			auth.Secure = true
		} else {
			opts = append(opts, grpc.WithInsecure())
		}
		opts = append(opts, grpc.WithPerRPCCredentials(auth))
		var err error
		cloud, err = cc.NewClient(target, opts...)
		if err != nil {
			cmd.Fatal(err)
		}
		buckets, err = bc.NewClient(target, opts...)
		if err != nil {
			cmd.Fatal(err)
		}
		threads, err = tc.NewClient(target, opts...)
		if err != nil {
			cmd.Fatal(err)
		}
	},
	PersistentPostRun: func(c *cobra.Command, args []string) {
		if cloud != nil {
			if err := cloud.Close(); err != nil {
				cmd.Fatal(err)
			}
		}
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current user",
	Long:  `Show the user for the current session.`,
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		who, err := cloud.Whoami(ctx)
		if err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("You are %s", aurora.White(who.Email).Bold())
	},
}

func authCtx(duration time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	ctx = common.NewSessionContext(ctx, authViper.GetString("session"))
	ctx = common.NewOrgNameContext(ctx, configViper.GetString("org"))
	ctx = common.NewThreadIDContext(ctx, getThreadID())
	return ctx, cancel
}

func getThreadID() (id thread.ID) {
	idstr := configViper.GetString("thread")
	if idstr != "" {
		var err error
		id, err = thread.Decode(idstr)
		if err != nil {
			cmd.Fatal(err)
		}
	}
	return
}
