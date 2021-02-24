package cli

import (
	"context"
	"math/big"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/cmd"
)

var filIndexCmd = &cobra.Command{
	Use:     "index",
	Aliases: []string{"index", "idx"},
	Short:   "Interact with the Miner Index.",
	Long:    `Interact with the Miner Index.`,
	Args:    cobra.ExactArgs(0),
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

		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
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
