package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/api/usersd/pb"
	"github.com/textileio/textile/v2/cmd"
)

var retrievalsStatusPrettyName = map[pb.ArchiveRetrievalStatus]string{
	pb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_UNSPECIFIED:  "Unspecified",
	pb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_QUEUED:       "Queued",
	pb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_EXECUTING:    "Executing",
	pb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_MOVETOBUCKET: "Moving to bucket",
	pb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_FAILED:       "Failed",
	pb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_CANCELED:     "Canceled",
	pb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_SUCCESS:      "Success",
}

var retrievalsCmd = &cobra.Command{
	Use:   "retrievals",
	Short: "Manages Filecoin retrievals.",
	Long:  `Manages Filecoin retrievals.`,
	Args:  cobra.ExactArgs(0),
}

var retrievalsLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List all created Filecoin retrievals.",
	Long:  `List all created Filecoin retrievals.`,
	Args:  cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		rs, err := clients.Users.ArchiveRetrievalLs(Auth(c.Context()))
		cmd.ErrCheck(err)

		if len(rs.Retrievals) > 0 {
			data := make([][]string, len(rs.Retrievals))
			for i, r := range rs.Retrievals {
				data[i] = []string{
					r.Id,
					r.Cid,
					retrievalsStatusPrettyName[r.Status],
					time.Unix(r.CreatedAt, 0).Format("02-Jan-06 15:04 -0700"),
					r.FailureCause,
				}
			}
			cmd.RenderTable([]string{"Id", "Cid", "Status", "Created", "Failure cause"}, data)
		}
		cmd.Message("Found %d retrievals", aurora.White(len(rs.Retrievals)).Bold())
	},
}

var retrievalsLogsCmd = &cobra.Command{
	Use:   "logs id",
	Short: "Show the logs of a retrieval.",
	Long:  `Show the logs of a retrieval.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		id := args[0]

		msgs := make(chan string)
		var err error
		go func() {
			err = clients.Users.ArchiveRetrievalLogs(Auth(c.Context()), id, msgs)
			close(msgs)
		}()

		for msg := range msgs {
			cmd.Message(msg)
		}

		if err != nil {
			// Any stream-based API closes after a perioid
			// or inacitvity. Show a nice message if that's the case.
			if strings.Contains(err.Error(), "RST_STREAM") {
				cmd.Warn("No news from this retrieval for a long-time. Re-run the command if you're still interested!")
				return
			}
			cmd.Fatal(err)
		}

	},
}
