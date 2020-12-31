package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
	"google.golang.org/protobuf/encoding/protojson"
)

var defaultArchiveConfigCmd = &cobra.Command{
	Use:   "default-config",
	Short: "Print the default archive storage configuration for the specified Bucket.",
	Long:  `Print the default archive storage configuration for the specified Bucket.`,
	Args:  cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		config, err := buck.DefaultArchiveConfig(ctx)
		cmd.ErrCheck(err)
		bytes, err := json.MarshalIndent(config, "", "  ")
		cmd.ErrCheck(err)
		cmd.Message("%s", string(bytes))
	},
}

var setDefaultArchiveConfigCmd = &cobra.Command{
	Use:   "set-default-config [(optional)file]",
	Short: "Set the default archive storage configuration for the specified Bucket from a file or stdin.",
	Long:  `Set the default archive storage configuration for the specified Bucket from a file or stdin.`,
	Args:  cobra.RangeArgs(0, 1),
	Run: func(c *cobra.Command, args []string) {
		var reader io.Reader
		if len(args) > 0 {
			file, err := os.Open(args[0])
			defer func() {
				err := file.Close()
				cmd.ErrCheck(err)
			}()
			reader = file
			cmd.ErrCheck(err)
		} else {
			reader = c.InOrStdin()
		}

		buf := new(bytes.Buffer)
		_, err := buf.ReadFrom(reader)
		cmd.ErrCheck(err)

		config := local.ArchiveConfig{}
		cmd.ErrCheck(json.Unmarshal(buf.Bytes(), &config))

		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		err = buck.SetDefaultArchiveConfig(ctx, config)
		cmd.ErrCheck(err)
		cmd.Success("Bucket default archive config updated")
	},
}

var archiveCmd = &cobra.Command{
	Use:     "archive",
	Aliases: []string{"archives"},
	Short:   "Create a Filecoin archive",
	Long:    `Creates a Filecoin archive from the remote bucket root. Pass in a custom archive storage config via the --file flag or stdin to override the default archive storage configuration.`,
	Args:    cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		yes, err := c.Flags().GetBool("yes")
		cmd.ErrCheck(err)
		if !yes {
			cmd.Warn("Archives are Filecoin Mainnet. Use with caution.")
			prompt := promptui.Prompt{
				Label:     "Proceed",
				IsConfirm: true,
			}
			if _, err := prompt.Run(); err != nil {
				cmd.End("")
			}
		}

		var reader io.Reader
		if c.Flags().Changed("file") {
			configPath, err := c.Flags().GetString("file")
			cmd.ErrCheck(err)
			file, err := os.Open(configPath)
			defer func() {
				err := file.Close()
				cmd.ErrCheck(err)
			}()
			cmd.ErrCheck(err)
			reader = file
			if !yes {
				cmd.Warn("ArchiveConfig properties RepFactor, ExcludedMiners, TrustedMiners and CountryCodes are currently ignored in Filecoin mainnet.")
				prompt := promptui.Prompt{
					Label:     "Proceed",
					IsConfirm: true,
				}
				if _, err := prompt.Run(); err != nil {
					cmd.End("")
				}
			}
		} else {
			stat, _ := os.Stdin.Stat()
			// stdin is being piped in (not being read from terminal)
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				reader = c.InOrStdin()
			}
		}

		var opts []local.ArchiveRemoteOption

		if reader != nil {
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(reader)
			cmd.ErrCheck(err)

			config := local.ArchiveConfig{}
			cmd.ErrCheck(json.Unmarshal(buf.Bytes(), &config))

			opts = append(opts, local.WithArchiveConfig(config))
		}

		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		err = buck.ArchiveRemote(ctx, opts...)
		cmd.ErrCheck(err)
		cmd.Success("Archive queued successfully")
	},
}

var archivesCmd = &cobra.Command{
	Use:   "list",
	Short: "Shows information about current and historical archives.",
	Long:  `Shows information about current and historical archives.`,
	Args:  cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		res, err := buck.Archives(ctx)
		cmd.ErrCheck(err)

		json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
		cmd.ErrCheck(err)

		cmd.Success("\n%v", string(json))
	},
}

var archiveWatchCmd = &cobra.Command{
	Use:   "watch",
	Short: "Watch the status of the most recent bucket archive.",
	Long:  `Watch the status of the most recent bucket archive.`,
	Args:  cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.ArchiveWatchTimeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		msgs, err := buck.ArchiveWatch(ctx)
		cmd.ErrCheck(err)
		for m := range msgs {
			switch m.Type {
			case local.ArchiveMessage:
				cmd.Message(m.Message)
			case local.ArchiveError:
				if m.InactivityClose {
					cmd.Warn("No news from this job for a long-time. Re-run the command if you're still interested!")
					break
				}
				cmd.Fatal(m.Error)
			}
		}
	},
}
