package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	pb "github.com/textileio/textile/api/buckets/pb"
	"github.com/textileio/textile/cmd"
)

var bucketArchiveCmd = &cobra.Command{
	Use:   "archive",
	Short: "Create a Filecoin archive",
	Long:  `Creates a Filecoin archive from the remote bucket root.`,
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(config.Viper, config.Flags)
		if config.Viper.ConfigFileUsed() == "" {
			cmd.Fatal(errNotABucket)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := clients.Ctx.Thread(cmd.Timeout)
		defer cancel()
		key := config.Viper.GetString("key")
		if _, err := clients.Buckets.Archive(ctx, key); err != nil {
			cmd.Fatal(err)
		}
		cmd.Success("Archive queued successfully")
	},
}

var bucketArchiveStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of the latest archive",
	Long:  `Shows the status of the most recent bucket archive.`,
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(config.Viper, config.Flags)
		if config.Viper.ConfigFileUsed() == "" {
			cmd.Fatal(errNotABucket)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := clients.Ctx.Thread(cmd.Timeout)
		defer cancel()
		key := config.Viper.GetString("key")
		r, err := clients.Buckets.ArchiveStatus(ctx, key)
		if err != nil {
			cmd.Fatal(err)
		}
		switch r.GetStatus() {
		case pb.ArchiveStatusReply_Failed:
			cmd.Warn("Archive failed with message: %s", r.GetFailedMsg())
		case pb.ArchiveStatusReply_Canceled:
			cmd.Warn("Archive was superseded by a new executing archive")
		case pb.ArchiveStatusReply_Executing:
			cmd.Message("Archive is currently executing, grab a coffee and be patient...")
		case pb.ArchiveStatusReply_Done:
			cmd.Success("Archive executed successfully!")
		default:
			cmd.Warn("Archive status unknown")
		}
		watch, err := c.Flags().GetBool("watch")
		if err != nil {
			cmd.Fatal(err)
		}
		if watch {
			fmt.Printf("\n")
			cmd.Message("Cid logs:")
			ch := make(chan string)
			wCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				err = clients.Buckets.ArchiveWatch(wCtx, key, ch)
				close(ch)
			}()
			for msg := range ch {
				cmd.Message("\t %s", msg)
				sctx, scancel := context.WithTimeout(context.Background(), cmd.TimeoutArchiveStatus)
				r, err := clients.Buckets.ArchiveStatus(sctx, key)
				if err != nil {
					cmd.Fatal(err)
				}
				scancel()
				if isJobStatusFinal(r.GetStatus()) {
					cancel()
				}
			}
			if err != nil {
				cmd.Fatal(err)
			}
		}
	},
}

func isJobStatusFinal(status pb.ArchiveStatusReply_Status) bool {
	switch status {
	case pb.ArchiveStatusReply_Failed, pb.ArchiveStatusReply_Canceled, pb.ArchiveStatusReply_Done:
		return true
	case pb.ArchiveStatusReply_Executing:
		return false
	}
	cmd.Fatal(fmt.Errorf("unknown job status"))
	return true

}

var bucketArchiveInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show info about the current archive",
	Long:  `Shows information about the current archive.`,
	PreRun: func(c *cobra.Command, args []string) {
		cmd.ExpandConfigVars(config.Viper, config.Flags)
		if config.Viper.ConfigFileUsed() == "" {
			cmd.Fatal(errNotABucket)
		}
	},
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := clients.Ctx.Thread(cmd.Timeout)
		defer cancel()
		key := config.Viper.GetString("key")
		r, err := clients.Buckets.ArchiveInfo(ctx, key)
		if err != nil {
			cmd.Fatal(err)
		}
		cmd.Message("Archive of Cid %s has %d deals:\n", r.Archive.Cid, len(r.Archive.Deals))
		var data [][]string
		for _, d := range r.Archive.GetDeals() {
			data = append(data, []string{d.ProposalCid, d.Miner})
		}
		cmd.RenderTable([]string{"ProposalCid", "Miner"}, data)

	},
}
