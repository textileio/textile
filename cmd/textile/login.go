package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/caarlos0/spin"
	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(loginCmd)
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login",
	Long:  `Login to Textile.`,
	Run: func(c *cobra.Command, args []string) {
		prompt := promptui.Prompt{
			Label: "Enter your username or email",
		}
		usernameOrEmail, err := prompt.Run()
		if err != nil {
			cmd.End("")
		}

		cmd.Message("We sent an email to the account address. Please follow the steps provided inside it.")

		s := spin.New("%s Waiting for your confirmation")
		s.Start()

		ctx, cancel := authCtx(confirmTimeout)
		defer cancel()
		res, err := hub.Signin(ctx, usernameOrEmail)
		s.Stop()
		if err != nil {
			cmd.Fatal(err)
		}

		authViper.Set("session", res.Session)

		home, err := homedir.Dir()
		if err != nil {
			cmd.Fatal(err)
		}
		dir := filepath.Join(home, ".textile")
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			cmd.Fatal(err)
		}

		filename := filepath.Join(dir, "auth.yml")

		if err := authViper.WriteConfigAs(filename); err != nil {
			cmd.Fatal(err)
		}

		fmt.Println(aurora.Sprintf("%s Email confirmed", aurora.Green("✔")))
		cmd.Success("You are now logged in. Initialize a new bucket with `%s`.",
			aurora.Cyan("textile buckets init"))
	},
}
