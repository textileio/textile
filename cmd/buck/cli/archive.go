package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
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
	Short: "Set the default archive storage configuration for the specified Bucket.",
	Long: `Set the default archive storage configuration for the specified Bucket from a file, stdin, or flags.

If flags are specified, this command updates the current default storage-config with the *explicitly set* flags. 
Flags that aren't explicitly set won't set the default value, and thus keep the original value in the storage-config.

If a file or stdin is used, the storage-config will be completely overridden by the provided one.`,
	Example: "hub buck archive set-default-config --rep-factor=3 --fast-retrieval --verified-deal --trusted-miners=f08240,f023467,f09848",
	Args:    cobra.RangeArgs(0, 1),
	Run: func(c *cobra.Command, args []string) {
		var config local.ArchiveConfig
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)

		stdIn, err := c.Flags().GetBool("stdin")
		cmd.ErrCheck(err)
		if stdIn {
			// Read config json from stdin
			reader := c.InOrStdin()
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(reader)
			cmd.ErrCheck(err)

			cmd.ErrCheck(json.Unmarshal(buf.Bytes(), &config))
		} else if len(args) > 0 {
			// Read config json from path
			file, err := os.Open(args[0])
			defer func() {
				err := file.Close()
				cmd.ErrCheck(err)
			}()
			reader := file
			cmd.ErrCheck(err)
			buf := new(bytes.Buffer)
			_, err = buf.ReadFrom(reader)
			cmd.ErrCheck(err)

			cmd.ErrCheck(json.Unmarshal(buf.Bytes(), &config))
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
			defer cancel()
			buck, err := bucks.GetLocalBucket(ctx, conf)
			cmd.ErrCheck(err)
			config, err = buck.DefaultArchiveConfig(ctx)
			cmd.ErrCheck(err)

			if c.Flags().Changed("rep-factor") {
				repFactor, err := c.Flags().GetInt("rep-factor")
				cmd.ErrCheck(err)
				config.RepFactor = repFactor
			}

			if c.Flags().Changed("deal-min-duration") {
				dealMinDuration, err := c.Flags().GetInt64("deal-min-duration")
				cmd.ErrCheck(err)
				config.DealMinDuration = dealMinDuration
			}

			if c.Flags().Changed("max-price") {
				maxPrice, err := c.Flags().GetUint64("max-price")
				cmd.ErrCheck(err)
				config.MaxPrice = maxPrice
			}

			if c.Flags().Changed("excluded-miners") {
				excludedMiners, err := c.Flags().GetStringSlice("excluded-miners")
				cmd.ErrCheck(err)
				config.ExcludedMiners = excludedMiners
			}

			if c.Flags().Changed("trusted-miners") {
				trustedMiners, err := c.Flags().GetStringSlice("trusted-miners")
				cmd.ErrCheck(err)
				config.TrustedMiners = trustedMiners
			}

			if c.Flags().Changed("country-codes") {
				countryCodes, err := c.Flags().GetStringSlice("country-codes")
				cmd.ErrCheck(err)
				config.CountryCodes = countryCodes
			}

			if c.Flags().Changed("fast-retrieval") {
				fastRetrieval, err := c.Flags().GetBool("fast-retrieval")
				cmd.ErrCheck(err)
				config.FastRetrieval = fastRetrieval
			}

			if c.Flags().Changed("verified-deal") {
				verifiedDeal, err := c.Flags().GetBool("verified-deal")
				cmd.ErrCheck(err)
				config.VerifiedDeal = verifiedDeal
			}

			if c.Flags().Changed("deal-start-offset") {
				dealStartOffset, err := c.Flags().GetInt64("deal-start-offset")
				cmd.ErrCheck(err)
				config.DealStartOffset = dealStartOffset
			}
		}

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
	Use:   "archive",
	Short: "Create a Filecoin archive",
	Long:  `Creates a Filecoin archive from the remote bucket root. Pass in a custom archive storage config via the --file flag or stdin to override the default archive storage configuration.`,
	Args:  cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		yes, err := c.Flags().GetBool("yes")
		cmd.ErrCheck(err)
		if !yes {
			cmd.Warn("The archive will be done in the Filecoin Mainnet. Use with caution.")
			prompt := promptui.Prompt{
				Label:     "Proceed",
				IsConfirm: true,
			}
			if _, err := prompt.Run(); err != nil {
				cmd.End("")
			}
			fmt.Println("")
		}

		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)

		diff, err := buck.DiffLocal()
		cmd.ErrCheck(err)
		if len(diff) != 0 && !yes {
			cmd.Warn("You have unpushed local changes. Are you sure you want to archive the last pushed bucket?")
			prompt := promptui.Prompt{
				Label:     "Proceed",
				IsConfirm: true,
			}
			if _, err := prompt.Run(); err != nil {
				cmd.End("")
			}
			fmt.Println("")
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

		opts := []local.ArchiveRemoteOption{local.WithSkipAutomaticVerifiedDeal(true)}

		config := local.ArchiveConfig{}
		if reader != nil {
			buf := new(bytes.Buffer)
			_, err := buf.ReadFrom(reader)
			cmd.ErrCheck(err)
			cmd.ErrCheck(json.Unmarshal(buf.Bytes(), &config))
		} else {
			config, err = buck.DefaultArchiveConfig(ctx)
			cmd.ErrCheck(err)
		}

		addrs, err := buck.Addresses(ctx)
		cmd.ErrCheck(err)
		if len(addrs.Addresses) != 1 {
			cmd.Fatal(fmt.Errorf("There should be exactly one wallet address but there are %d", len(addrs.Addresses)))
		}

		addrInfo := addrs.Addresses[0]
		balance, ok := big.NewInt(0).SetString(addrInfo.Balance, 10)
		if !ok {
			cmd.Fatal(fmt.Errorf("parsing current balance"))
		}
		if balance.Cmp(big.NewInt(0)) == 0 {
			cmd.Fatal(fmt.Errorf("The wallet address balance is zero, you'll need to add some funds!"))
		}

		skipVerifiedDealOverride, err := c.Flags().GetBool("skip-verified-deal-override")
		cmd.ErrCheck(err)
		if !skipVerifiedDealOverride {
			if !config.VerifiedDeal && addrInfo.VerifiedClientInfo != nil {
				remainingDataCap, ok := big.NewInt(0).SetString(addrInfo.VerifiedClientInfo.RemainingDatacapBytes, 10)
				if !ok {
					cmd.Fatal(fmt.Errorf("Parsing remaining datacap"))
				}
				if remainingDataCap.Cmp(big.NewInt(0)) > 0 {
					// If the default storage-config is !verified-deal, but the client
					// is verified, then help him set this value automatically.
					cmd.Message("The Filecoin wallet is verified, enabling verified deals automatically.")
					config.VerifiedDeal = true
				} else {
					cmd.Message("The Filecoin wallet is verified, but the remaining data-cap is zero.")
				}
			} else if !config.VerifiedDeal && addrInfo.VerifiedClientInfo == nil {
				// If the client isn't verified, we can't tune its storage-config
				// to get a verified deal. Explain how to get automatically verified.
				cmd.Warn("The Filecoin wallet address isn't verified, which can potentially lead to paying high-prices for storage.")
				cmd.Message("You can get verified automatically using your GitHub account at https://verify.glif.io")
			} else if config.VerifiedDeal && addrInfo.VerifiedClientInfo == nil {
				// Explain to the user that despite she has set the verifiedDeal
				// attribute in it's storage config, it needs to get verified first.
				cmd.Warn("Despite the storage-config has verified-deals enabled, the Filecoin wallet address isn't verified.")
				cmd.Message("You can get verified automatically using your GitHub account at https://verify.glif.io")
			}
		}

		if !config.VerifiedDeal && !yes {
			cmd.Warn("Are you sure you want to archive with an unverified deal?")
			prompt := promptui.Prompt{
				Label:     "Proceed",
				IsConfirm: true,
			}
			if _, err := prompt.Run(); err != nil {
				cmd.End("")
			}
			fmt.Println("")
		}
		if len(config.TrustedMiners) == 0 && !yes {
			cmd.Message("The storage-config trusted miners list is empty, which can lead to unreliable and high-cost Filecoin deal making.")
			cmd.Message("You can discover reliable miners by exploring: `hub fil query` and `hub fil calculate`.")
			cmd.Warn("Are you sure want to use potentially unreliable miners?")
			prompt := promptui.Prompt{
				Label:     "Proceed",
				IsConfirm: true,
			}
			if _, err := prompt.Run(); err != nil {
				cmd.End("")
			}
			fmt.Println("")
		}

		if config.MaxPrice == 0 && !yes {
			cmd.Warn("The storage-config doesn't specify a limit to pay for storage price")
			cmd.Message("You can set a limit with `hub buck set-storage-config --set-max-price`")
			cmd.Warn("Are you sure you want to make a deal without a max-price limit?")
			prompt := promptui.Prompt{
				Label:     "Proceed",
				IsConfirm: true,
			}
			if _, err := prompt.Run(); err != nil {
				cmd.End("")
			}
			fmt.Println("")
		}

		opts = append(opts, local.WithArchiveConfig(config))
		err = buck.ArchiveRemote(ctx, opts...)
		cmd.ErrCheck(err)

		cmd.Success("Archive queued successfully")
	},
}

var archiveLsCmd = &cobra.Command{
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
