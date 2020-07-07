package cli

import (
	"fmt"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	pb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/uiprogress"
)

const (
	Name = "buck"

	nonFastForwardMsg = "the root of your bucket is behind (try `%s` before pushing again)"
)

var (
	config = cmd.Config{
		Viper: viper.New(),
		Dir:   ".textile",
		Name:  "config",
		Flags: map[string]cmd.Flag{
			"key": {
				Key:      "key",
				DefValue: "",
			},
			"org": {
				Key:      "org",
				DefValue: "",
			},
			"thread": {
				Key:      "thread",
				DefValue: "",
			},
		},
		EnvPre: "BUCK",
		Global: false,
	}

	clients *cmd.Clients

	addFileTimeout = time.Hour * 24
	getFileTimeout = time.Hour * 24

	errNotABucket = fmt.Errorf("not a bucket (or any of the parent directories): .textile")
)

func init() {
	uiprogress.Empty = ' '
	uiprogress.Fill = '-'
}

func Init(rootCmd *cobra.Command) {
	rootCmd.AddCommand(bucketInitCmd, bucketLinksCmd, bucketRootCmd, bucketStatusCmd, bucketLsCmd, bucketPushCmd, bucketPullCmd, bucketAddCmd, bucketCatCmd, bucketDestroyCmd, bucketEncryptCmd, bucketDecryptCmd, bucketArchiveCmd)
	bucketArchiveCmd.AddCommand(bucketArchiveStatusCmd, bucketArchiveInfoCmd)

	bucketInitCmd.PersistentFlags().String("key", "", "Bucket key")
	bucketInitCmd.PersistentFlags().String("org", "", "Org username")
	bucketInitCmd.PersistentFlags().String("thread", "", "Thread ID")
	if err := cmd.BindFlags(config.Viper, bucketInitCmd, config.Flags); err != nil {
		cmd.Fatal(err)
	}
	bucketInitCmd.Flags().StringP("name", "n", "", "Bucket name")
	bucketInitCmd.Flags().BoolP("private", "p", false, "Obfuscates files and folders with encryption")
	bucketInitCmd.Flags().String("cid", "", "Bootstrap the bucket with a UnixFS Cid from the IPFS network")
	bucketInitCmd.Flags().BoolP("existing", "e", false, "Initializes from an existing remote bucket if true")

	bucketPushCmd.Flags().BoolP("force", "f", false, "Allows non-fast-forward updates if true")
	bucketPushCmd.Flags().BoolP("yes", "y", false, "Skips the confirmation prompt if true")
	bucketPushCmd.Flags().Int64("maxsize", buckMaxSizeMiB, "Max bucket size in MiB")

	bucketPullCmd.Flags().BoolP("force", "f", false, "Force pull all remote files if true")
	bucketPullCmd.Flags().Bool("hard", false, "Pulls and prunes local changes if true")
	bucketPullCmd.Flags().BoolP("yes", "y", false, "Skips the confirmation prompt if true")

	bucketAddCmd.Flags().BoolP("yes", "y", false, "Skips confirmations prompts to always overwrite files and merge folders")

	bucketEncryptCmd.Flags().StringP("password", "p", "", "Encryption password")
	bucketDecryptCmd.Flags().StringP("password", "p", "", "Decryption password")

	bucketArchiveStatusCmd.Flags().BoolP("watch", "w", false, "Watch execution log")
}

func Config() cmd.Config {
	return config
}

func SetClients(c *cmd.Clients) {
	clients = c
}

func printLinks(reply *pb.LinksReply) {
	cmd.Message("Your bucket links:")
	cmd.Message("%s Thread link", aurora.White(reply.URL).Bold())
	cmd.Message("%s IPNS link (propagation can be slow)", aurora.White(reply.IPNS).Bold())
	if reply.WWW != "" {
		cmd.Message("%s Bucket website", aurora.White(reply.WWW).Bold())
	}
}

func setCidVersion(buck *local.Bucket, key string) {
	_, rc, err := buck.Root()
	if err != nil {
		cmd.Fatal(err)
	}
	if !rc.Defined() {
		buck.SetCidVersion(int(getRemoteRoot(key).Version()))
	}
}
