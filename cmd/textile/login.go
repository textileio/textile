package main

import (
	"context"
	"fmt"
	"net/mail"
	"os"
	"path"

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
			Label: "Enter your email",
			Validate: func(email string) error {
				_, err := mail.ParseAddress(email)
				return err
			},
		}
		email, err := prompt.Run()
		if err != nil {
			log.Fatal(err)
		}

		// @todo: Add a security code that can be visually verified in the email.
		cmd.Message("We sent an email to %s. Please follow the steps provided inside it.",
			aurora.White(email).Bold())

		s := spin.New("%s Waiting for your confirmation")
		s.Start()

		ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)
		defer cancel()
		res, err := client.Login(ctx, email)
		s.Stop()
		if err != nil {
			cmd.Fatal(err)
		}

		authViper.Set("token", res.SessionID)

		home, err := homedir.Dir()
		if err != nil {
			cmd.Fatal(err)
		}
		dir := path.Join(home, ".textile")
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			cmd.Fatal(err)
		}

		filename := path.Join(dir, "auth.yml")

		if err := authViper.WriteConfigAs(filename); err != nil {
			cmd.Fatal(err)
		}

		fmt.Println(aurora.Sprintf("%s Email confirmed", aurora.Green("✔")))
		cmd.Success("You are now logged in. Initialize a new project directory with `%s`.",
			aurora.Cyan("textile init"))
	},
}
