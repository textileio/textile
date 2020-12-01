package cli

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/caarlos0/spin"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/cmd"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize account",
	Long:  `Initializes a new Hub account.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		usernamePrompt := promptui.Prompt{
			Label: "Choose a username",
			Templates: &promptui.PromptTemplates{
				Valid: fmt.Sprintf("%s {{ . | bold }}%s ", cmd.Bold(promptui.IconInitial), cmd.Bold(":")),
			},
		}
		username, err := usernamePrompt.Run()
		if err != nil {
			cmd.End("")
		}
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		err = clients.Hub.IsUsernameAvailable(ctx, username)
		cmd.ErrCheck(err)

		emailPrompt := promptui.Prompt{
			Label: "Enter your email",
			Validate: func(email string) error {
				_, err := mail.ParseAddress(email)
				return err
			},
		}
		email, err := emailPrompt.Run()
		if err != nil {
			cmd.End("")
		}

		cmd.Message("We sent an email to %s. Please follow the steps provided inside it.",
			aurora.White(email).Bold())
		s := spin.New("%s Waiting for your confirmation")
		s.Start()

		cctx, ccancel := context.WithTimeout(context.Background(), confirmTimeout)
		defer ccancel()
		res, err := clients.Hub.Signup(cctx, username, email)
		s.Stop()
		if err != nil {
			if strings.Contains(err.Error(), "Account exists") {
				cmd.Fatal(fmt.Errorf("an account with email %s already exists, use `%s login` instead", email, Name))
			} else {
				cmd.Fatal(err)
			}
			return
		}
		config.Viper.Set("session", res.Session)

		cmd.WriteConfigToHome(config)

		fmt.Println(aurora.Sprintf("%s Email confirmed", aurora.Green("✔")))
		cmd.Success("Welcome to the Hub. Initialize a new bucket with `%s`.", aurora.Cyan(Name+" buck init"))
	},
}
