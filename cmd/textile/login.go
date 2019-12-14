package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(loginCmd)

	loginCmd.Flags().String(
		"username",
		"",
		"Username")
	loginCmd.Flags().String(
		"email",
		"",
		"Email")
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login",
	Long:  `Login to Textile.`,
	Run: func(c *cobra.Command, args []string) {
		id, err := client.SignUp()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Println(id)
	},
}
