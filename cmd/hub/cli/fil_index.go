package cli

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	ccodes "github.com/launchdarkly/go-country-codes"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/api/mindexd/pb"
	"github.com/textileio/textile/v2/cmd"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func init() {
	filQueryMiners.Flags().Bool("ascending", false, "sort results ascending, default is sort descending")
	filQueryMiners.Flags().Int64("limit", 10, "maximum results per page")
	filQueryMiners.Flags().Int64("offset", 0, "number of results to skip")
	filQueryMiners.Flags().String("sort-field", "textile-deals-last-successful", "sort field")
	filQueryMiners.Flags().String("sort-textile-region", "", "make the sorting criteria in a specified region")
	filQueryMiners.Flags().String("filter-miner-location", "", "filter by miner's location")
	filQueryMiners.Flags().Bool("show-full-details", false, "indicates that the results will contain extended data about miners")
	filQueryMiners.Flags().Bool("json", false, "indicates that the output should be printed in JSON")

	filGetMinerInfo.Flags().Bool("json", false, "indicates that output should be printed in JSON")
}

var m49Table = map[string]string{
	"021": "North America",
}

var filIndexCmd = &cobra.Command{
	Use:     "index",
	Aliases: []string{"index", "idx"},
	Short:   "Interact with the Miner Index.",
	Long:    `Interact with the Miner Index.`,
	Args:    cobra.ExactArgs(0),
}

var filQueryMiners = &cobra.Command{
	Use:   "query",
	Short: "Query miners in the index",
	Long: `Query miners in the index.
	The API allows to handle paging with filters and sorting.
	The sort-field flag can take the following values:
	- "textile-deals-total-successful": Total successful deals in Textile.
	- "textile-deals-last-successful": Last successful deal in Textile.
	- "ask-price": Raw ask-price.
	- "verified-ask-price": Verified ask-price.
	- "active-sectors": Total active sectors in the network.

	The default sorting field is "textile-deals-last-successful".
	The sort-textile-region allows to apply textile-deals-* flags to a specific 
	textile region. An empty value would be interpreted to the aggregate of all
	regions (default).

	If the flag --show-full-details is set, an extra set of columns with more details
	is printed.

	If the flag --json is set, the output will be printed in JSON and it always contains
	full details.
	`,
	Args: cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ascending, err := c.Flags().GetBool("ascending")
		cmd.ErrCheck(err)
		limit, err := c.Flags().GetInt64("limit")
		cmd.ErrCheck(err)
		offset, err := c.Flags().GetInt64("offset")
		cmd.ErrCheck(err)
		sortField, err := c.Flags().GetString("sort-field")
		cmd.ErrCheck(err)
		sortTextileRegion, err := c.Flags().GetString("sort-textile-region")
		cmd.ErrCheck(err)
		filterMinerLocation, err := c.Flags().GetString("filter-miner-location")
		cmd.ErrCheck(err)

		req := &pb.QueryIndexRequest{
			Filters: &pb.QueryIndexRequestFilters{
				MinerLocation: filterMinerLocation,
			},
			Sort: &pb.QueryIndexRequestSort{
				Ascending:     ascending,
				TextileRegion: sortTextileRegion,
				Field:         mapSortField(sortField),
			},
			Limit:  int32(limit),
			Offset: offset,
		}
		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		res, err := clients.MinerIndex.QueryIndex(ctx, req)
		cmd.ErrCheck(err)

		rawJSON, err := c.Flags().GetBool("json")
		cmd.ErrCheck(err)
		if rawJSON {
			json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
			cmd.ErrCheck(err)
			fmt.Println(string(json))
			return
		}

		showFullDetails, err := c.Flags().GetBool("show-full-details")
		cmd.ErrCheck(err)
		fmt.Printf("%s\n", aurora.Bold(aurora.Green("-- Results --")))
		data := make([][]string, len(res.Miners))
		for i, m := range res.Miners {
			if showFullDetails {
				data[i] = []string{
					m.Miner.MinerAddr,
					fmt.Sprintf("%s%%", humanize.FtoaWithDigits(m.Miner.Filecoin.RelativePower, 5)),
					fmt.Sprintf("%s | %s", attoFilToFil(m.Miner.Filecoin.AskPrice), attoFilToFil(m.Miner.Filecoin.AskVerifiedPrice)),
					fmt.Sprintf("%s | %s", humanize.IBytes(uint64(m.Miner.Filecoin.MinPieceSize)), humanize.IBytes(uint64(m.Miner.Filecoin.MaxPieceSize))),
					fmt.Sprintf("%s", humanize.IBytes(uint64(m.Miner.Filecoin.SectorSize))),
					fmt.Sprintf("%d | %d", m.Miner.Filecoin.ActiveSectors, m.Miner.Filecoin.FaultySectors),
					isoLocationToCountryName(m.Miner.Metadata.Location),
					fmt.Sprintf("%d | %d", m.Miner.Textile.DealsSummary.Total, m.Miner.Textile.DealsSummary.Failures),
					fmt.Sprintf("%s", prettyFormatPbTime(m.Miner.Textile.DealsSummary.Last)),
					fmt.Sprintf("%s", peekAndFormatTransferTimes(m.Miner.Textile)),
					fmt.Sprintf("%s", peekAndFormatSealingTimes(m.Miner.Textile)),
				}
				continue
			}
			data[i] = []string{
				m.Miner.MinerAddr,
				fmt.Sprintf("%s", attoFilToFil(m.Miner.Filecoin.AskPrice)),
				fmt.Sprintf("%s", attoFilToFil(m.Miner.Filecoin.AskVerifiedPrice)),
				isoLocationToCountryName(m.Miner.Metadata.Location),
				fmt.Sprintf("%s", humanize.IBytes(uint64(m.Miner.Filecoin.MinPieceSize))),
				fmt.Sprintf("%s", prettyFormatPbTime(m.Miner.Textile.DealsSummary.Last)),
			}
		}
		if showFullDetails {
			cmd.RenderTable([]string{"miner", "relative power", "raw/verified ask price", "min/max piece size", "sector size", "active/faulty sectors", "location", "textile total/failed deals", "last successful", "last data-transfer", "last sealing-time"}, data)
		} else {
			cmd.RenderTable([]string{"miner", "ask-price (FIL/GiB/epoch)", "verified ask-price (FIL/GiB/epoch)", "location", "min-piece-size", "textile-last-deal"}, data)
		}
	},
}

func peekAndFormatTransferTimes(ti *pb.TextileInfo) string {
	var last *time.Time
	var lastThroughput *float64
	for _, r := range ti.Regions {
		if len(r.Deals.TailTransfers) > 0 {
			if last == nil || last.Before(r.Deals.TailTransfers[0].TransferedAt.AsTime()) {
				lastThroughput = &r.Deals.TailTransfers[0].MibPerSec
				t := r.Deals.TailTransfers[0].TransferedAt.AsTime()
				last = &t
			}
		}
	}

	if last == nil {
		return ""
	}

	return fmt.Sprintf("~%.02f MiB/s (%s)", *lastThroughput, humanize.Time(*last))
}

func peekAndFormatSealingTimes(ti *pb.TextileInfo) string {
	var last *time.Time
	var lastDuration *float64
	for _, r := range ti.Regions {
		if len(r.Deals.TailSealed) > 0 {
			if last == nil || last.Before(r.Deals.TailSealed[0].SealedAt.AsTime()) {
				durationHours := time.Duration(time.Second * time.Duration(r.Deals.TailSealed[0].DurationSeconds)).Hours()
				t := r.Deals.TailSealed[0].SealedAt.AsTime()
				lastDuration = &durationHours
				last = &t
			}
		}
	}

	if last == nil {
		return ""
	}

	return fmt.Sprintf("~%.0f hours (%s)", *lastDuration, humanize.Time(*last))
}

var filGetMinerInfo = &cobra.Command{
	Use:   "get [minerAddr]",
	Short: "Get miner information",
	Long:  `Get miner information`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		minerAddr := args[0]

		ctx, cancel := context.WithTimeout(context.Background(), cmd.Timeout)
		defer cancel()
		res, err := clients.MinerIndex.GetMinerInfo(ctx, minerAddr)
		cmd.ErrCheck(err)

		rawJSON, err := c.Flags().GetBool("json")
		cmd.ErrCheck(err)
		if rawJSON {
			json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
			cmd.ErrCheck(err)
			fmt.Println(string(json))
			return
		}

		fmt.Printf("%s\n\n", aurora.Bold("Miner "+res.Info.MinerAddr))

		fmt.Printf("%s\n", aurora.Bold(aurora.Green("-- Filecoin --")))
		cmd.Message("Relative Power: %s%%", humanize.FtoaWithDigits(res.Info.Filecoin.RelativePower, 5))
		cmd.Message("Location: %s", isoLocationToCountryName(res.Info.Metadata.Location))
		cmd.Message("Sector size   : %s", humanize.IBytes(uint64(res.Info.Filecoin.SectorSize)))
		cmd.Message("Active sectors: %d", res.Info.Filecoin.ActiveSectors)
		cmd.Message("Faulty sectors: %d\n", res.Info.Filecoin.FaultySectors)

		cmd.Message("Unverified Price: %s FIL/GiB/epoch", attoFilToFil(res.Info.Filecoin.AskPrice))
		cmd.Message("Verified Price  : %s FIL/GiB/epoch\n", attoFilToFil(res.Info.Filecoin.AskPrice))
		cmd.Message("Minimum Piece Size: %s", humanize.IBytes(uint64(res.Info.Filecoin.MinPieceSize)))
		cmd.Message("Maximum Piece Size: %s\n", humanize.IBytes(uint64(res.Info.Filecoin.MaxPieceSize)))

		fmt.Printf("%s\n", aurora.Bold(aurora.Green("-- Textile --")))
		cmd.Message("Total successful deals: %d", res.Info.Textile.DealsSummary.Total)
		cmd.Message("Last successful deal  : %s", prettyFormatPbTime(res.Info.Textile.DealsSummary.Last))
		cmd.Message("Total failed deals: %d", res.Info.Textile.DealsSummary.Failures)
		cmd.Message("Last failed deal  : %s\n", prettyFormatPbTime(res.Info.Textile.DealsSummary.LastFailure))

		for m49Code, regionData := range res.Info.Textile.Regions {
			var dataTransfers [][]string
			for _, t := range regionData.Deals.TailTransfers {
				dataTransfers = append(dataTransfers, []string{
					fmt.Sprintf("%s", prettyFormatPbTime(t.TransferedAt)),
					fmt.Sprintf("~%.02f MiB/s", t.MibPerSec),
				})
			}
			var sealingDuration [][]string
			for _, s := range regionData.Deals.TailSealed {
				sealingDuration = append(sealingDuration, []string{
					fmt.Sprintf("%s", prettyFormatPbTime(s.SealedAt)),
					fmt.Sprintf("~%.0f hours", (time.Second * time.Duration(s.DurationSeconds)).Hours()),
				})
			}

			fmt.Print(aurora.Brown(fmt.Sprintf("%s deals telemetry:\n", m49Table[m49Code])))
			cmd.Message("Total successful: %d", regionData.Deals.Total)
			cmd.Message("Last successful : %s", prettyFormatPbTime(regionData.Deals.Last))
			cmd.Message("Total failed    : %d", regionData.Deals.Failures)
			cmd.Message("Last failed     : %s\n", prettyFormatPbTime(regionData.Deals.LastFailure))
			cmd.RenderTableWithoutNewLines([]string{"date", "datatransfer-speed"}, dataTransfers)
			fmt.Println()
			cmd.RenderTableWithoutNewLines([]string{"date", "sealing-duration"}, sealingDuration)
		}
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

		fmt.Printf("%s\n", aurora.Bold(aurora.Green("-- Calculated prices --")))
		data := make([][]string, len(res.Results))
		for i, p := range res.Results {
			data[i] = []string{
				p.Miner,
				fmt.Sprintf("%s (%s FIL/GiB/epoch)", attoFilToFil(p.TotalCost), attoFilToFil(p.Price)),
				fmt.Sprintf("%s (%s FIL/GiB/epoch)", attoFilToFil(p.VerifiedTotalCost), attoFilToFil(p.VerifiedPrice)),
			}
		}
		cmd.Message("Padded size: %s", humanize.IBytes(uint64(res.PaddedSize)))
		cmd.Message("Duration in epochs: %d", res.DurationEpochs)
		cmd.RenderTable([]string{"miner", "cost", "verified-client cost"}, data)

		fmt.Printf("%s\n", aurora.Bold(aurora.Green("-- Buckets --")))
		for _, p := range res.Results {
			cmd.Message("hub buck archive set-default-config --fast-retrieval --trusted-miners %s", p.Miner)
			cmd.Message("hub buck archive set-default-config --fast-retrieval --verified-deal --trusted-miners %s", p.Miner)
			fmt.Println()
		}

		fmt.Printf("\n%s\n", aurora.Bold(aurora.Green("-- Lotus CLI --")))
		for _, p := range res.Results {
			cmd.Message("lotus client deal --fast-retrieval <data-cid> %s %s %d", p.Miner, p.TotalCost, res.DurationEpochs)
			cmd.Message("lotus client deal --fast-retrieval --verified-deal <data-cid> %s %s %d", p.Miner, p.VerifiedTotalCost, res.DurationEpochs)
			fmt.Println()
		}
		cmd.Message("Remember that to make verified deals, your wallet address should be verified: https://plus.fil.org/")
	},
}

func attoFilToFil(attoFil string) string {
	if attoFil == "" {
		return "<none>"
	}
	attoBig, _ := big.NewInt(0).SetString(attoFil, 10)
	r := new(big.Rat).SetFrac(attoBig, big.NewInt(1_000_000_000_000_000_000))
	if r.Sign() == 0 {
		return "0 FIL" // Note(jsign): do we need to be conservative here about free-cost?
	}
	return strings.TrimRight(strings.TrimRight(r.FloatString(18), "0"), ".") + " FIL"
}

func prettyFormatPbTime(t *timestamppb.Timestamp) string {
	if t == nil {
		return "<none>"
	}

	ti := t.AsTime()
	return prettyFormatTime(&ti)
}

func prettyFormatTime(t *time.Time) string {
	if t == nil {
		return "<none>"
	}

	return fmt.Sprintf("%s (%s)", t.Format("2006-01-02 15:04:05"), humanize.Time(*t))
}

func mapSortField(field string) pb.QueryIndexRequestSortField {
	switch field {
	case "textile-deals-total-successful":
		return pb.QueryIndexRequestSortField_TEXTILE_DEALS_TOTAL_SUCCESSFUL
	case "textile-deals-last-successful":
		return pb.QueryIndexRequestSortField_TEXTILE_DEALS_LAST_SUCCESSFUL
	case "ask-price":
		return pb.QueryIndexRequestSortField_ASK_PRICE
	case "verified-ask-price":
		return pb.QueryIndexRequestSortField_VERIFIED_ASK_PRICE
	case "active-sectors":
		return pb.QueryIndexRequestSortField_ACTIVE_SECTORS
	default:
		return pb.QueryIndexRequestSortField_TEXTILE_DEALS_LAST_SUCCESSFUL
	}
}

func isoLocationToCountryName(isoCode string) string {
	c, ok := ccodes.GetByAlpha2(isoCode)
	if !ok {
		return isoCode
	}

	return c.Name
}
