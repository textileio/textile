package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/dcrypto"
	"github.com/textileio/textile/cmd"
)

var bucketEncryptCmd = &cobra.Command{
	Use:   "encrypt [file] [password]",
	Short: "Encrypt file with a password",
	Long:  `Encrypts file with a password.`,
	Args:  cobra.ExactArgs(2),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(config.Viper, config.Flags)
		if config.Viper.ConfigFileUsed() == "" {
			cmd.Fatal(errNotABucket)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		file, err := os.Open(args[0])
		if err != nil {
			cmd.Fatal(err)
		}
		defer file.Close()
		info, err := file.Stat()
		if err != nil {
			cmd.Fatal(err)
		}
		if info.IsDir() {
			cmd.Fatal(fmt.Errorf("path %s is not a file", args[0]))
		}

		r, err := dcrypto.NewEncrypterWithPassword(file, []byte(args[1]))
		if err != nil {
			cmd.Fatal(err)
		}
		if _, err := io.Copy(os.Stdout, r); err != nil {
			cmd.Fatal(err)
		}
	},
}

func getPassword(c *cobra.Command, label string) string {
	pass, err := c.Flags().GetString("password")
	if err != nil {
		cmd.Fatal(err)
	}
	if pass == "" {
		namep := promptui.Prompt{
			Label: label,
			Mask:  '*',
			Validate: func(s string) error {
				if len(s) == 0 {
					return fmt.Errorf("invalid password")
				}
				return nil
			},
		}
		var err error
		pass, err = namep.Run()
		if err != nil {
			cmd.End("")
		}
	}
	return pass
}
