package cli

import (
	"strconv"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/cmd"
)

var archivesCmd = &cobra.Command{
	Use:   "archives",
	Short: "Manages account-wide Filecoin archives.",
	Long:  `Manages account-wide Filecoin archives.`,
	Args:  cobra.ExactArgs(0),
}

var archivesLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "Lists all known archived data in the Filecoin network.",
	Long:  `List all known archive data in the Filecoin network. This includes made bucket archives, or imported deals.`,
	Args:  cobra.NoArgs,
	Run: func(c *cobra.Command, args []string) {
		as, err := clients.Users.ArchivesLs(Auth(c.Context()))
		cmd.ErrCheck(err)

		if len(as.Archives) > 0 {
			data := make([][]string, len(as.Archives))
			for i, a := range as.Archives {
				dealIDsStr := make([]string, len(a.Info))
				for i, inf := range a.Info {
					dealIDsStr[i] = strconv.FormatUint(inf.DealId, 10)
				}
				data[i] = []string{
					a.Cid,
					strings.Join(dealIDsStr, ","),
				}
			}
			cmd.RenderTable([]string{"Cid", "Deal IDs"}, data)
		}
		cmd.Message("Found %d account archives", aurora.White(len(as.Archives)).Bold())
	},
}

var archivesImportCmd = &cobra.Command{
	Use:   "import", // cid [deal-id ...]",
	Short: "Imports a list of active deal-ids from the Filecoin network for a Cid.",
	Long:  "Imports a list of active deal-ids from the Filecoin network for a Cid. At least one deal-id must be provided. Cids must be UnixFS DAGs.",
	Args:  cobra.MinimumNArgs(2),
	Run: func(c *cobra.Command, args []string) {
		dataCid, err := cid.Decode(args[0])
		cmd.ErrCheck(err)

		dealIDs := make([]uint64, len(args)-1)
		for i := 0; i < len(args)-1; i++ {
			dealIDs[i], err = strconv.ParseUint(args[i+1], 10, 64)
			cmd.ErrCheck(err)
		}

		err = clients.Users.ArchivesImport(Auth(c.Context()), dataCid, dealIDs)
		cmd.ErrCheck(err)

		cmd.Success("Deals imported successfully")
	},
}
