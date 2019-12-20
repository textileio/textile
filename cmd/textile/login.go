package main

import (
	"context"
	"net/mail"
	"os"
	"path"

	"github.com/caarlos0/spin"
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

		s := spin.New("%s please verify your email...")
		s.Start()

		ctx := context.Background()
		token, err := client.Login(ctx, email, loginTimeout)
		s.Stop()

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
