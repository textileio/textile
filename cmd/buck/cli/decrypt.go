package cli

import (
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/textileio/dcrypto"
	"github.com/textileio/textile/cmd"
)

var bucketDecryptCmd = &cobra.Command{
	Use:   "decrypt [path] [password]",
	Short: "Decrypt bucket objects at path with password",
	Long:  `Decrypts bucket objects at path with the given password and writes to stdout.`,
	Args:  cobra.ExactArgs(2),
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(config.Viper, config.Flags)
		if config.Viper.ConfigFileUsed() == "" {
			cmd.Fatal(errNotABucket)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		reader, writer := io.Pipe()
		go func() {
			ctx, cancel := clients.Ctx.Thread(getFileTimeout)
			defer cancel()
			key := config.Viper.GetString("key")
			if err := clients.Buckets.PullPath(ctx, key, args[0], writer); err != nil {
				cmd.Fatal(err)
			}
			if err := writer.Close(); err != nil {
				cmd.Fatal(err)
			}
		}()

		r, err := dcrypto.NewDecrypterWithPassword(reader, []byte(args[1]))
		if err != nil {
			cmd.Fatal(err)
		}
		defer r.Close()
		if _, err := io.Copy(os.Stdout, r); err != nil {
			cmd.Fatal(err)
		}
	},
}
