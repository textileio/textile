package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"
	userPb "github.com/textileio/powergate/api/gen/powergate/user/v1"
	"github.com/textileio/textile/v2/cmd"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	powStorageCmd.Flags().BoolP("ascending", "a", false, "sort records ascending, default is sort descending")
	powStorageCmd.Flags().StringSlice("cids", []string{}, "limit the records to deals for the specified data cids, treated as and AND operation if --addrs is also provided")
	powStorageCmd.Flags().StringSlice("addrs", []string{}, "limit the records to deals initiated from  the specified wallet addresses, treated as and AND operation if --cids is also provided")
	powStorageCmd.Flags().BoolP("include-pending", "p", false, "include pending deals")
	powStorageCmd.Flags().BoolP("include-final", "f", true, "include final deals")

	powRetrievalsCmd.Flags().BoolP("ascending", "a", false, "sort records ascending, default is sort descending")
	powRetrievalsCmd.Flags().StringSlice("cids", []string{}, "limit the records to deals for the specified data cids, treated as and AND operation if --addrs is also provided")
	powRetrievalsCmd.Flags().StringSlice("addrs", []string{}, "limit the records to deals initiated from  the specified wallet addresses, treated as and AND operation if --cids is also provided")
}

var powCmd = &cobra.Command{
	Use:   "pow",
	Short: "Interact with Powergate",
	Long:  `Interact with Powergate.`,
	Args:  cobra.ExactArgs(0),
}

var powAddrsCmd = &cobra.Command{
	Use:   "addrs",
	Short: "List Filecoin wallet addresses associated with the current account or org",
	Long:  `List Filecoin wallet addresses associated with the current account or org.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		res, err := clients.Pow.Addresses(ctx)
		cmd.ErrCheck(err)
		json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", string(json))
	},
}

var powBalanceCmd = &cobra.Command{
	Use:   "balance [addr]",
	Short: "Display the FIL balance of a wallet address",
	Long:  `Display the FIL balance of a wallet address.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		res, err := clients.Pow.Balance(ctx, args[0])
		cmd.ErrCheck(err)
		json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", string(json))
	},
}

var powInfoCmd = &cobra.Command{
	Use:   "info [optional cid1,cid2,...]",
	Short: "Get information about the current storate state of a cid",
	Long:  `Get information about the current storate state of a cid`,
	Args:  cobra.MaximumNArgs(1),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		var cids []string
		if len(args) > 0 {
			cids = strings.Split(args[0], ",")
		}
		res, err := clients.Pow.CidInfo(ctx, cids...)
		cmd.ErrCheck(err)
		json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", string(json))
	},
}

var powStorageCmd = &cobra.Command{
	Use:   "storage",
	Short: "List Powergate storage deal records associated with the current account or org",
	Long:  `List Powergate storage deal records associated with the current account or org.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		ascending, err := c.Flags().GetBool("ascending")
		cmd.ErrCheck(err)
		cids, err := c.Flags().GetStringSlice("cids")
		cmd.ErrCheck(err)
		addrs, err := c.Flags().GetStringSlice("addrs")
		cmd.ErrCheck(err)
		pending, err := c.Flags().GetBool("include-pending")
		cmd.ErrCheck(err)
		final, err := c.Flags().GetBool("include-final")
		cmd.ErrCheck(err)
		conf := &userPb.DealRecordsConfig{
			Ascending:      ascending,
			DataCids:       cids,
			FromAddrs:      addrs,
			IncludeFinal:   final,
			IncludePending: pending,
		}
		res, err := clients.Pow.StorageDealRecords(ctx, conf)
		cmd.ErrCheck(err)
		json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", string(json))
	},
}

var powRetrievalsCmd = &cobra.Command{
	Use:   "retrievals",
	Short: "List Powergate retrieval deal records associated with the current account or org",
	Long:  `List Powergate retrieval deal records associated with the current account or org.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		ascending, err := c.Flags().GetBool("ascending")
		cmd.ErrCheck(err)
		cids, err := c.Flags().GetStringSlice("cids")
		cmd.ErrCheck(err)
		addrs, err := c.Flags().GetStringSlice("addrs")
		cmd.ErrCheck(err)
		conf := &userPb.DealRecordsConfig{
			Ascending: ascending,
			DataCids:  cids,
			FromAddrs: addrs,
		}
		res, err := clients.Pow.RetrievalDealRecords(ctx, conf)
		cmd.ErrCheck(err)
		json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", string(json))
	},
}
