package cli

import (
	"context"
	"os"
	"runtime"
	"strconv"

	aurora2 "github.com/logrusorgru/aurora"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
)

const Name = "buck"

var bucks *local.Buckets

var aurora = aurora2.NewAurora(runtime.GOOS != "windows")

type Format string

const (
	DefaultFormat Format = "default"
	JSONFormat           = "json"
)

func Init(baseCmd *cobra.Command) {
	baseCmd.AddCommand(
		initCmd,
		getCmd,
		existingCmd,
		linksCmd,
		rootCmd,
		statusCmd,
		lsCmd,
		pushCmd,
		pullCmd,
		addCmd,
		watchCmd,
		catCmd,
		destroyCmd,
		encryptCmd,
		decryptCmd,
		archiveCmd,
		rolesCmd,
	)
	archiveCmd.AddCommand(defaultArchiveConfigCmd, setDefaultArchiveConfigCmd, archiveWatchCmd, archiveLsCmd)
	rolesCmd.AddCommand(rolesGrantCmd, rolesLsCmd)

	baseCmd.PersistentFlags().String("key", "", "Bucket key")
	baseCmd.PersistentFlags().String("thread", "", "Thread ID")

	initCmd.Flags().StringP("name", "n", "", "Bucket name")
	initCmd.Flags().BoolP("private", "p", false, "Obfuscates files and folders with encryption")
	initCmd.Flags().String("cid", "", "Bootstrap the bucket with a UnixFS Cid from the IPFS network")
	initCmd.Flags().BoolP("existing", "e", false, "Interactively select an existing remote bucket if true")
	initCmd.Flags().Bool("soft", false, "Accepts all local changes, including deletions, if true")
	initCmd.Flags().Bool("hard", false, "Discards all local changes if true")
	initCmd.Flags().BoolP("yes", "y", false, "Skips the confirmation prompt if true")
	initCmd.Flags().BoolP("quiet", "q", false, "Write minimal output")
	// (jsign): disabled until this feature is usable in mainnet.
	// initCmd.Flags().Bool("unfreeze", false, "Unfreeze --cid from a known or imported deals in Filecoin.")

	pushCmd.Flags().BoolP("force", "f", false, "Allows non-fast-forward updates if true")
	pushCmd.Flags().BoolP("yes", "y", false, "Skips the confirmation prompt if true")
	pushCmd.Flags().BoolP("quiet", "q", false, "Write minimal output")

	pullCmd.Flags().BoolP("force", "f", false, "Force pull all remote files if true")
	pullCmd.Flags().Bool("hard", false, "Discards local changes if true")
	pullCmd.Flags().BoolP("yes", "y", false, "Skips the confirmation prompt if true")
	pullCmd.Flags().BoolP("quiet", "q", false, "Write minimal output")

	addCmd.Flags().BoolP("yes", "y", false, "Skips confirmations prompts to always overwrite files and merge folders")

	encryptCmd.Flags().StringP("password", "p", "", "Encryption password")
	decryptCmd.Flags().StringP("password", "p", "", "Decryption password")

	archiveCmd.Flags().StringP("file", "f", "", "Optional path to a file containing archive config json that will override the default")
	archiveCmd.Flags().BoolP("yes", "y", false, "Skips the confirmation prompt if true")
	archiveCmd.Flags().Bool("skip-verified-deal-override", false, "Skips automatic set of verified-deal in storage-config")

	setDefaultArchiveConfigCmd.Flags().IntP("rep-factor", "r", 1, "Target number of active deals")
	setDefaultArchiveConfigCmd.Flags().Int64P("deal-min-duration", "d", 0, "Minimum duration for the deal")
	setDefaultArchiveConfigCmd.Flags().StringSliceP("excluded-miners", "x", nil, "Miner addresses that should not be used")
	setDefaultArchiveConfigCmd.Flags().StringSliceP("trusted-miners", "t", nil, "Miner addresses that must be used")
	setDefaultArchiveConfigCmd.Flags().StringSliceP("country-codes", "c", nil, "Select miners with specific countries")
	setDefaultArchiveConfigCmd.Flags().Uint64P("max-price", "p", 0, "Maximum price that will be spent per RepFactor in AttoFIL/GiB per Epoch")
	setDefaultArchiveConfigCmd.Flags().BoolP("fast-retrieval", "f", true, "Created deals should enable the fast retrieval feature")
	setDefaultArchiveConfigCmd.Flags().BoolP("verified-deal", "v", false, "Deal is originating from a verified client with DataCap")
	setDefaultArchiveConfigCmd.Flags().Int64P("deal-start-offset", "e", 0, "Epochs in the future impose a deadline for deals to be on-chain")
	setDefaultArchiveConfigCmd.Flags().BoolP("stdin", "i", false, "Set config from stdin JSON")

	rolesGrantCmd.Flags().StringP("role", "r", "", "Access role: none, reader, writer, admin")

	linksCmd.Flags().String("format", "default", "Display URL links in the provided format. Options: [default,json]")
}

func SetBucks(b *local.Buckets) {
	bucks = b
}

var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get a bucket",
	Long:  `Gets bucket metadata.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		info, err := buck.Info(ctx)
		cmd.ErrCheck(err)
		cmd.JSON(info)
	},
}

var existingCmd = &cobra.Command{
	Use:   "existing",
	Short: "List buckets",
	Long:  `Lists all buckets.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		list, err := bucks.RemoteBuckets(ctx, conf.Thread)
		cmd.ErrCheck(err)
		var data [][]string
		if len(list) > 0 {
			for _, item := range list {
				data = append(data, []string{
					item.Name,
					item.Thread.String(),
					item.Key,
					item.Path.Cid().String(),
				})
			}
		}
		if len(data) > 0 {
			cmd.RenderTable([]string{"name", "thread", "key", "root"}, data)
		}
		cmd.Message("Found %d buckets", aurora.White(len(data)).Bold())
	},
}

var statusCmd = &cobra.Command{
	Use: "status",
	Aliases: []string{
		"st",
	},
	Short: "Show bucket object changes",
	Long:  `Displays paths that have been added to and paths that have been removed or differ from the local bucket root.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		diff, err := buck.DiffLocal()
		cmd.ErrCheck(err)
		if len(diff) == 0 {
			cmd.End("Everything up-to-date")
		}
		for _, c := range diff {
			cf := local.ChangeColor(c.Type)
			cmd.Message("%s  %s", cf(local.ChangeType(c.Type)), cf(c.Rel))
		}
	},
}

var rootCmd = &cobra.Command{
	Use:   "root",
	Short: "Show bucket root CIDs",
	Long:  `Shows the local and remote bucket root CIDs (these will differ if the bucket is encrypted).`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		r, err := buck.Roots(ctx)
		cmd.ErrCheck(err)
		if r.Local.Defined() {
			cmd.Message("%s (local)", aurora.White(r.Local).Bold())
		}
		cmd.Message("%s (remote)", aurora.White(r.Remote).Bold())
	},
}

var linksCmd = &cobra.Command{
	Use: "links [path]",
	Aliases: []string{
		"link",
	},
	Short: "Display URL links to a bucket object.",
	Long:  `Displays a thread, IPNS, and website link to a bucket object. Omit path to display the top-level links.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		var pth string
		if len(args) > 0 {
			pth = args[0]
		}
		links, err := buck.RemoteLinks(ctx, pth)
		cmd.ErrCheck(err)

		format, err := c.Flags().GetString("format")
		cmd.ErrCheck(err)

		printLinks(links, Format(format))
	},
}

func printLinks(reply local.Links, format Format) {
	switch format {
	case JSONFormat:
		cmd.JSON(reply)
	default:
		cmd.Message("Your bucket links:")
		cmd.Message("%s Thread link", aurora.White(reply.URL).Bold())
		cmd.Message("%s IPNS link (propagation can be slow)", aurora.White(reply.IPNS).Bold())
		if reply.WWW != "" {
			cmd.Message("%s Bucket website", aurora.White(reply.WWW).Bold())
		}
	}
}

var lsCmd = &cobra.Command{
	Use: "ls [path]",
	Aliases: []string{
		"list",
	},
	Short: "List top-level or nested bucket objects",
	Long:  `Lists top-level or nested bucket objects.`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		var pth string
		if len(args) > 0 {
			pth = args[0]
		}
		items, err := buck.ListRemotePath(ctx, pth)
		cmd.ErrCheck(err)
		var data [][]string
		if len(items) > 0 {
			for _, item := range items {
				var links string
				if item.IsDir {
					links = strconv.Itoa(item.ItemsCount)
				} else {
					links = "n/a"
				}
				data = append(data, []string{
					item.Name,
					formatBytes(item.Size, false),
					strconv.FormatBool(item.IsDir),
					links,
					item.Cid.String(),
				})
			}
		}
		if len(data) > 0 {
			cmd.RenderTable([]string{"name", "size", "dir", "objects", "cid"}, data)
		}
		cmd.Message("Found %d objects", aurora.White(len(data)).Bold())
	},
}

var catCmd = &cobra.Command{
	Use:   "cat [path]",
	Short: "Cat bucket objects at path",
	Long:  `Cats bucket objects at path.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.PullTimeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		err = buck.CatRemotePath(ctx, args[0], os.Stdout)
		cmd.ErrCheck(err)
	},
}

var encryptCmd = &cobra.Command{
	Use:   "encrypt [file] [password]",
	Short: "Encrypt file with a password",
	Long:  `Encrypts file with a password (WARNING: Password is not recoverable).`,
	Args:  cobra.ExactArgs(2),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		err = buck.EncryptLocalPathWithPassword(args[0], args[1], os.Stdout)
		cmd.ErrCheck(err)
	},
}

var decryptCmd = &cobra.Command{
	Use:   "decrypt [path] [password]",
	Short: "Decrypt bucket objects at path with password",
	Long:  `Decrypts bucket objects at path with the given password and writes to stdout.`,
	Args:  cobra.ExactArgs(2),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.PullTimeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		err = buck.DecryptRemotePathWithPassword(ctx, args[0], args[1], os.Stdout)
		cmd.ErrCheck(err)
	},
}

var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy bucket and all objects",
	Long:  `Destroys the bucket and all objects.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		conf, err := bucks.NewConfigFromCmd(c, ".")
		cmd.ErrCheck(err)
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		buck, err := bucks.GetLocalBucket(ctx, conf)
		cmd.ErrCheck(err)
		cmd.Warn("%s",
			aurora.Red(
				"This action cannot be undone. The bucket and all associated data will be permanently deleted."))
		prompt := promptui.Prompt{
			Label:     "Are you absolutely sure",
			IsConfirm: true,
		}
		if _, err = prompt.Run(); err != nil {
			cmd.End("")
		}
		err = buck.Destroy(ctx)
		cmd.ErrCheck(err)
		cmd.Success("Your bucket has been deleted")
	},
}
