package cli

import (
	"context"
	"encoding/hex"

	"github.com/spf13/cobra"
	userPb "github.com/textileio/powergate/v2/api/gen/powergate/user/v1"
	"github.com/textileio/textile/v2/cmd"
	"google.golang.org/protobuf/encoding/protojson"
)

func init() {
	filStorageCmd.Flags().BoolP("ascending", "a", false, "sort records ascending, default is sort descending")
	filStorageCmd.Flags().StringSlice("cids", []string{}, "limit the records to deals for the specified data cids, treated as and AND operation if --addrs is also provided")
	filStorageCmd.Flags().StringSlice("addrs", []string{}, "limit the records to deals initiated from  the specified wallet addresses, treated as and AND operation if --cids is also provided")
	filStorageCmd.Flags().BoolP("include-pending", "p", false, "include pending deals")
	filStorageCmd.Flags().BoolP("include-final", "f", true, "include final deals")

	filRetrievalsCmd.Flags().BoolP("ascending", "a", false, "sort records ascending, default is sort descending")
	filRetrievalsCmd.Flags().StringSlice("cids", []string{}, "limit the records to deals for the specified data cids, treated as and AND operation if --addrs is also provided")
	filRetrievalsCmd.Flags().StringSlice("addrs", []string{}, "limit the records to deals initiated from  the specified wallet addresses, treated as and AND operation if --cids is also provided")
}

const addrsWarning = "Funds in this wallet are for network and storage fees only; they cannot be transferred or sold."
const addrsGetVerified = "Get your address verified on https://plus.fil.org/landing."

var filCmd = &cobra.Command{
	Use:     "fil",
	Aliases: []string{"filecoin", "pow"},
	Short:   "Interact with Filecoin related commands.",
	Long:    `Interact with Filecoin related commands.`,
	Args:    cobra.ExactArgs(0),
}

var filAddrsCmd = &cobra.Command{
	Use:   "addrs",
	Short: "List Filecoin wallet addresses associated with the current account or org",
	Long:  `List Filecoin wallet addresses associated with the current account or org.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		res, err := clients.Filecoin.Addresses(ctx)
		cmd.ErrCheck(err)
		json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", string(json))
		cmd.Message(addrsWarning)
		// provide link for verification
		showVerificationInfo := true
		for _, addr := range res.Addresses {
			if addr.VerifiedClientInfo != nil {
				showVerificationInfo = false
			}
		}
		if showVerificationInfo {
			cmd.Message(addrsGetVerified)
		}
	},
}

var filBalanceCmd = &cobra.Command{
	Use:   "balance [addr]",
	Short: "Display the FIL balance of a wallet address",
	Long:  `Display the FIL balance of a wallet address.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		res, err := clients.Filecoin.Balance(ctx, args[0])
		cmd.ErrCheck(err)
		json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", string(json))
		cmd.Message(addrsWarning)
	},
}

var filSignCmd = &cobra.Command{
	Use:   "sign [hex-encoded-message]",
	Short: "Signs a message with user wallet addresses.",
	Long:  "Signs a message using all wallet addresses associated with the user",
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()

		b, err := hex.DecodeString(args[0])
		cmd.ErrCheck(err)

		res, err := clients.Filecoin.Addresses(ctx)
		cmd.ErrCheck(err)

		data := make([][]string, len(res.Addresses))
		for i, a := range res.Addresses {
			signRes, err := clients.Filecoin.SignMessage(ctx, a.Address, b)
			cmd.ErrCheck(err)
			data[i] = []string{a.Address, hex.EncodeToString(signRes.Signature)}
		}

		cmd.RenderTable([]string{"address", "signature"}, data)
	},
}

var filVerifyCmd = &cobra.Command{
	Use:   "verify [addr] [hex-encoded-message] [hex-encoded-signature]",
	Short: "Verifies the signature of a message signed with a user wallet address.",
	Long:  "Verifies the signature of a message signed with a user wallet address.",
	Args:  cobra.ExactArgs(3),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()

		mb, err := hex.DecodeString(args[1])
		cmd.ErrCheck(err)
		sb, err := hex.DecodeString(args[2])
		cmd.ErrCheck(err)

		res, err := clients.Filecoin.VerifyMessage(ctx, args[0], mb, sb)
		cmd.ErrCheck(err)

		json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
		cmd.ErrCheck(err)

		cmd.Success("\n%v", string(json))
	},
}

var filInfoCmd = &cobra.Command{
	Use:   "info cid",
	Short: "Get information about the current storage state of a cid",
	Long:  `Get information about the current storage state of a cid`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		cid := args[0]
		res, err := clients.Filecoin.CidInfo(ctx, cid)
		cmd.ErrCheck(err)
		json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", string(json))
	},
}

var filStorageCmd = &cobra.Command{
	Use:   "storage",
	Short: "List Filecoin storage deal records associated with the current account or org",
	Long:  `List Filecoin storage deal records associated with the current account or org.`,
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
		res, err := clients.Filecoin.StorageDealRecords(ctx, conf)
		cmd.ErrCheck(err)
		json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", string(json))
	},
}

var filRetrievalsCmd = &cobra.Command{
	Use:   "retrievals",
	Short: "List Filecoin retrieval deal records associated with the current account or org",
	Long:  `List Filecoin retrieval deal records associated with the current account or org.`,
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
		res, err := clients.Filecoin.RetrievalDealRecords(ctx, conf)
		cmd.ErrCheck(err)
		json, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", EmitUnpopulated: true}.Marshal(res)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", string(json))
	},
}
