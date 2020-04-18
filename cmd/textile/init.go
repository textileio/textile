package main

import (
	"fmt"
	"net/mail"
	"os"
	"path/filepath"

	"github.com/textileio/textile/util"

	"github.com/caarlos0/spin"
	"github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/cmd"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize account",
	Long:  `Initialize a new Textile account.`,
	Run: func(c *cobra.Command, args []string) {
		prompt1 := promptui.Prompt{
			Label: "Choose a username (letters and numbers only)",
			Validate: func(un string) error {
				ctx, cancel := authCtx(cmdTimeout)
				defer cancel()
				ok, err := hub.CheckUsername(ctx, un)
				if err != nil {
					cmd.Fatal(err)
				}
				if !ok {
					return fmt.Errorf("username taken")
				}
				return nil
			},
		}
		username, err := prompt1.Run()
		if err != nil {
			cmd.End("")
		}
		validUsername, err := util.ToValidName(username)
		if err != nil {
			cmd.Fatal(err)
		}
		prompt2 := promptui.Prompt{
			Label: "Enter your email",
			Validate: func(email string) error {
				_, err := mail.ParseAddress(email)
				return err
			},
		}
		email, err := prompt2.Run()
		if err != nil {
			cmd.End("")
		}
		prompt3 := promptui.Prompt{
			Label:     fmt.Sprintf("Username: %s\n Email: %s", validUsername, email),
			IsConfirm: true,
		}
		ok, err := prompt3.Run()
		if err != nil {
			cmd.End("")
		}
		fmt.Println(ok)
		//if !ok {
		//	cmd.End("")
		//}
		cmd.End("")

		cmd.Message("We sent an email to %s. Please follow the steps provided inside it.",
			aurora.White(email).Bold())

		s := spin.New("%s Waiting for your confirmation")
		s.Start()

		ctx, cancel := authCtx(confirmTimeout)
		defer cancel()
		res, err := hub.Signup(ctx, username, email)
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

		if err = authViper.WriteConfigAs(filename); err != nil {
			cmd.Fatal(err)
		}

		fmt.Println(aurora.Sprintf("%s Email confirmed", aurora.Green("✔")))
		cmd.Success("Welcome to Textile. Initialize a new bucket with `%s`.",
			aurora.Cyan("textile buckets init"))
	},
}
