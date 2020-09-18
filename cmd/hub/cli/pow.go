package cli

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/spf13/cobra"
	ffsRpc "github.com/textileio/powergate/ffs/rpc"
	"github.com/textileio/textile/cmd"
)

func init() {
	powNewAddrCmd.Flags().StringP("type", "t", "bls", "Wallet address type to create - bls or secp256k1. Defaults to bls.")
	powNewAddrCmd.Flags().BoolP("default", "d", false, "Whether to make the new addres the default. Defaults to false.")

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

var powHealthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check the health of the Powergate node",
	Long:  `Check the health of the Powergate node.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		res, err := clients.Pow.Health(ctx)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", proto.MarshalTextString(res))
	},
}

var powPeersCmd = &cobra.Command{
	Use:   "peers",
	Short: "List Powergate's Filecoin peers",
	Long:  `List Powergate's Filecoin peers.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		res, err := clients.Pow.Peers(ctx)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", proto.MarshalTextString(res))
	},
}

var powFindPeerCmd = &cobra.Command{
	Use:   "find-peer [peer-id]",
	Short: "Find a Filecoin peer by id",
	Long:  `Find a Filecoin peer by id.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		res, err := clients.Pow.FindPeer(ctx, args[0])
		cmd.ErrCheck(err)
		cmd.Success("\n%v", proto.MarshalTextString(res))
	},
}

var powConnectednessCmd = &cobra.Command{
	Use:   "connectedness [peer-id]",
	Short: "Get the connectedness state to a Filecoin peer",
	Long:  `Get the connectedness state to a Filecoin peer.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		res, err := clients.Pow.Connectedness(ctx, args[0])
		cmd.ErrCheck(err)
		cmd.Success("\n%v", proto.MarshalTextString(res))
	},
}

var powAddrsCmd = &cobra.Command{
	Use:   "addrs",
	Short: "List Filecoin wallet addresses associated with the current account or org",
	Long:  `List Filecoin wallet addresses associated with the current account or org.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		res, err := clients.Pow.Addrs(ctx)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", proto.MarshalTextString(res))
	},
}

var powNewAddrCmd = &cobra.Command{
	Use:   "new-addr [name]",
	Short: "Create a Filecoin wallet addresses associated with the current account or org",
	Long:  `Create a Filecoin wallet addresses associated with the current account or org.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		typ, err := c.Flags().GetString("type")
		cmd.ErrCheck(err)
		def, err := c.Flags().GetBool("default")
		cmd.ErrCheck(err)
		res, err := clients.Pow.NewAddr(ctx, args[0], typ, def)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", proto.MarshalTextString(res))
	},
}

var powInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display information about the Powergate associated with the current account or org to any other account",
	Long:  `Display information about the Powergate associated with the current account or org to any other account.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		res, err := clients.Pow.Info(ctx)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", proto.MarshalTextString(res))
	},
}

var powShowCmd = &cobra.Command{
	Use:   "show [cid]",
	Short: "Display information about a stored CID",
	Long:  `Display information about a stored CID.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		res, err := clients.Pow.Show(ctx, args[0])
		cmd.ErrCheck(err)
		cmd.Success("\n%v", proto.MarshalTextString(res))
	},
}

var powShowAllCmd = &cobra.Command{
	Use:   "show-all",
	Short: "Display information about all stored CIDs",
	Long:  `Display information about all stored CIDs.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		res, err := clients.Pow.ShowAll(ctx)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", proto.MarshalTextString(res))
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
		conf := &ffsRpc.ListDealRecordsConfig{
			Ascending:      ascending,
			DataCids:       cids,
			FromAddrs:      addrs,
			IncludeFinal:   final,
			IncludePending: pending,
		}
		res, err := clients.Pow.ListStorageDealRecords(ctx, conf)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", proto.MarshalTextString(res))
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
		conf := &ffsRpc.ListDealRecordsConfig{
			Ascending: ascending,
			DataCids:  cids,
			FromAddrs: addrs,
		}
		res, err := clients.Pow.ListRetrievalDealRecords(ctx, conf)
		cmd.ErrCheck(err)
		cmd.Success("\n%v", proto.MarshalTextString(res))
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
		cmd.Success("\n%v", proto.MarshalTextString(res))
	},
}
