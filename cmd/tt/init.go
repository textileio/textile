package main

import (
	"fmt"
	"net/mail"
	"os"
	"path/filepath"
	"strings"

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
	Long:  `Initialize a new Textile account (interactive).`,
	Run: func(c *cobra.Command, args []string) {
		prompt1 := promptui.Prompt{
			Label: "Choose a username",
			Templates: &promptui.PromptTemplates{
				Valid: fmt.Sprintf("%s {{ . | bold }}%s ", bold(promptui.IconInitial), bold(":")),
			},
		}
		username, err := prompt1.Run()
		if err != nil {
			cmd.End("")
		}
		ctx, cancel := authCtx(cmdTimeout)
		defer cancel()
		if err := hub.IsUsernameAvailable(ctx, username); err != nil {
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

		cmd.Message("We sent an email to %s. Please follow the steps provided inside it.",
			aurora.White(email).Bold())
		s := spin.New("%s Waiting for your confirmation")
		s.Start()

		ctx, cancel = authCtx(confirmTimeout)
		defer cancel()
		res, err := hub.Signup(ctx, username, email)
		s.Stop()
		if err != nil {
			if strings.Contains(err.Error(), "Account exists") {
				cmd.Fatal(fmt.Errorf("an account with email %s already exists, use `tt login` instead", email))
			} else {
				cmd.Fatal(err)
			}
			return
		}

		authViper.Set("session", res.Session)

		home, err := homedir.Dir()
		if err != nil {
			cmd.Fatal(err)
		}
		dir := filepath.Join(home, authDir)
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			cmd.Fatal(err)
		}

		filename := filepath.Join(dir, "auth.yml")

		if err = authViper.WriteConfigAs(filename); err != nil {
			cmd.Fatal(err)
		}

		fmt.Println(aurora.Sprintf("%s Email confirmed", aurora.Green("✔")))
		cmd.Success("Welcome to Textile. Initialize a new bucket with `%s`.", aurora.Cyan("tt bucket init"))
	},
}
