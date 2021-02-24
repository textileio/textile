package cli

import (
	"context"
	"math/big"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/cmd"
	"google.golang.org/protobuf/encoding/protojson"
)

var filIndexCmd = &cobra.Command{
	Use:     "index",
	Aliases: []string{"index", "idx"},
	Short:   "Interact with the Miner Index.",
	Long:    `Interact with the Miner Index.`,
	Args:    cobra.ExactArgs(0),
}

var filGetMinerInfo = &cobra.Command{
	Use:   "miner [minerAddr]",
	Short: "Get miner information",
	Long:  `Get miner information`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		minerAddr := args[0]

		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		res, err := clients.MinerIndex.GetMinerInfo(ctx, minerAddr)
		cmd.ErrCheck(err)

		json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", string(json))

		cmd.Message("%s\n", aurora.Bold("Miner "+res.Info.MinerAddr))

		cmd.Message("%s", aurora.Bold(aurora.Green("-- Filecoin information --")))
		cmd.Message("Relative Power: %s%%", humanize.FtoaWithDigits(res.Info.Filecoin.RelativePower, 5))
		cmd.Message("Sector size: %s\n", humanize.Bytes(uint64(res.Info.Filecoin.SectorSize)))

		cmd.Message("Verified-client Price  : %s FIL/GiB/epoch", attoFilToFil(res.Info.Filecoin.AskPrice))
		cmd.Message("Unverified-client Price: %s FIL/GiB/epoch\n", attoFilToFil(res.Info.Filecoin.AskPrice))

		cmd.Message("Minimum Piece Size: %s", humanize.Bytes(uint64(res.Info.Filecoin.MinPieceSize)))
		cmd.Message("Maximum Piece Size: %s\n", humanize.Bytes(uint64(res.Info.Filecoin.MaxPieceSize)))
	},
}

var filCalculateDealPrice = &cobra.Command{
	Use:   "calculate [dataSizeBytes] [durationDays] [minerAddr...]",
	Short: "Calculate deal prices for a list of miners.",
	Long:  `Calculate deal prices for a list of miners.`,
	Args:  cobra.MinimumNArgs(3),
	Run: func(c *cobra.Command, args []string) {
		dataSizeBytes, err := humanize.ParseBytes(args[0])
		cmd.ErrCheck(err)
		durationDays, err := strconv.ParseInt(args[1], 10, 64)
		cmd.ErrCheck(err)

		minersAddr := make([]string, len(args)-2)
		for i := 2; i < len(args); i++ {
			minersAddr[i-2] = args[i]
		}

		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		res, err := clients.MinerIndex.CalculateDealPrice(ctx, minersAddr, int64(dataSizeBytes), durationDays)
		cmd.ErrCheck(err)

		cmd.Message("Padded size: %s", humanize.IBytes(uint64(res.PaddedSize)))
		cmd.Message("Duration in epochs: %d", res.DurationEpochs)

		data := make([][]string, len(res.Results))
		for i, p := range res.Results {
			data[i] = []string{p.Miner, attoFilToFil(p.TotalCost), attoFilToFil(p.VerifiedTotalCost)}
		}

		cmd.RenderTable([]string{"miner", "cost", "verified-client cost"}, data)
	},
}

func attoFilToFil(attoFil string) string {
	attoBig, _ := big.NewInt(0).SetString(attoFil, 10)
	r := new(big.Rat).SetFrac(attoBig, big.NewInt(1_000_000_000_000_000_000))
	if r.Sign() == 0 {
		return "0 FIL" // Note(jsign): do we need to be conservative here about free-cost?
	}
	return strings.TrimRight(strings.TrimRight(r.FloatString(18), "0"), ".") + " FIL"
}
