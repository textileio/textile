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
)

var defaultArchiveConfigCmd = &cobra.Command{
	Use:   "default-config",
	Short: "Print the default archive storage configuration for the specified Bucket.",
	Long:  `Print the default archive storage configuration for the specified Bucket.`,
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, ".")
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

		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, ".")
		cmd.ErrCheck(err)
		err = buck.SetDefaultArchiveConfig(ctx, config)
		cmd.ErrCheck(err)
		cmd.Success("Bucket default archive config updated")
	},
}

var archiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Create a Filecoin archive",
	Long:  `Creates a Filecoin archive from the remote bucket root. Pass in a custom archive storage config via the --file flag or stdin to override the default archive storage configuration.`,
	Run: func(c *cobra.Command, args []string) {
		yes, err := c.Flags().GetBool("yes")
		cmd.ErrCheck(err)
		if !yes {
			cmd.Warn("Archives are currently saved on an experimental test network. They may be lost at any time.")
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

		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, ".")
		cmd.ErrCheck(err)
		err = buck.ArchiveRemote(ctx, opts...)
		cmd.ErrCheck(err)
		cmd.Success("Archive queued successfully")
	},
}

var archiveStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of the latest archive",
	Long:  `Shows the status of the most recent bucket archive.`,
	Run: func(c *cobra.Command, args []string) {
		watch, err := c.Flags().GetBool("watch")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.ArchiveWatchTimeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, ".")
		cmd.ErrCheck(err)
		msgs, err := buck.ArchiveStatus(ctx, watch)
		cmd.ErrCheck(err)
		for m := range msgs {
			switch m.Type {
			case local.ArchiveMessage:
				cmd.Message(m.Message)
			case local.ArchiveWarning:
				cmd.Warn(m.Message)
			case local.ArchiveError:
				if m.InactivityClose {
					cmd.Warn("No news from this job for a long-time. Re-run the command if you're still interested!")
					break
				}
				cmd.Fatal(m.Error)
			case local.ArchiveSuccess:
				cmd.Success(m.Message)
			}
		}
	},
}

var archiveInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show info about the current archive",
	Long:  `Shows information about the current archive.`,
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, ".")
		cmd.ErrCheck(err)
		info, err := buck.ArchiveInfo(ctx)
		cmd.ErrCheck(err)
		cmd.Message("Archive of cid %s has %d deals:\n", info.Archive.Cid, len(info.Archive.Deals))
		var data [][]string
		for _, d := range info.Archive.Deals {
			data = append(data, []string{d.ProposalCid.String(), d.Miner})
		}
		cmd.RenderTable([]string{"proposal cid", "miner"}, data)
	},
}
