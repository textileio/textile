package main

import (
	"net/mail"
	"os"
	"path"

	"github.com/textileio/textile/cmd"

	"github.com/mitchellh/go-homedir"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
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

		token, err := client.Login(email)
		if err != nil {
			log.Fatal(err)
		}
		authViper.Set("token", token)

		home, err := homedir.Dir()
		if err != nil {
			log.Fatal(err)
		}
		dir := path.Join(home, ".textile")
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			log.Fatal(err)
		}

		filename := path.Join(dir, "auth.yml")

		if err := authViper.WriteConfigAs(filename); err != nil {
			cmd.Fatal(err)
		}
	},
}
