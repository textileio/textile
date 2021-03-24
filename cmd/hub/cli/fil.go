package cli

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/spf13/cobra"
	userPb "github.com/textileio/powergate/v2/api/gen/powergate/user/v1"
	filrewardspb "github.com/textileio/textile/v2/api/filrewardsd/pb"
	hc "github.com/textileio/textile/v2/api/hubd/client"
	sendfilpb "github.com/textileio/textile/v2/api/sendfild/pb"
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

	filRewardsCmd.Flags().BoolP("ascending", "a", false, "sort results ascending by creation time")
	filRewardsCmd.Flags().BoolP("dev", "d", false, "filter results to rewards unlocked by the current dev")

	filClaimsCmd.Flags().BoolP("ascending", "a", false, "sort results ascending by creation time")
	filClaimsCmd.Flags().BoolP("dev", "d", false, "filter results to claims claimed by the current dev")
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

var filRewardsCmd = &cobra.Command{
	Use:   "rewards",
	Short: "List FIL rewards that have been unlocked for the specified dev and/or org",
	Long:  `List FIL rewards that have been unlocked for the specified dev and/or org.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()

		var opts []hc.ListFilRewardsOption

		ascending, err := c.Flags().GetBool("ascending")
		cmd.ErrCheck(err)
		dev, err := c.Flags().GetBool("dev")
		cmd.ErrCheck(err)

		if ascending {
			opts = append(opts, hc.ListFilRewardsAscending())
		}
		if dev {
			opts = append(opts, hc.ListFilRewardsUnlockedByDev())
		}

		// ToDo: Deal with paging.

		rewards, err := clients.Hub.ListFilRewards(ctx, opts...)
		cmd.ErrCheck(err)

		if len(rewards) > 0 {
			data := make([][]string, len(rewards))
			for i, reward := range rewards {
				nanofil := reward.BaseNanoFilReward * reward.Factor
				fil := fmt.Sprintf("%.8f", float64(nanofil)/math.Pow10(9))
				createdAt := reward.CreatedAt.AsTime().Format(time.RFC3339)
				t := filrewardspb.RewardType_name[int32(reward.Type)]
				data[i] = []string{t, fil, createdAt}
			}
			cmd.RenderTable([]string{"type", "amount (fil)", "created at"}, data)
		}
	},
}

var filClaimCmd = &cobra.Command{
	Use:   "claim [amount in FIL]",
	Short: "Claim unlocked FIL rewards to the specified org",
	Long:  `Claim unlocked FIL rewards to the specified org.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()

		fil, err := strconv.ParseFloat(args[0], 64)
		cmd.ErrCheck(err)
		nanofil := int64(fil * math.Pow10(9))

		err = clients.Hub.ClaimFil(ctx, nanofil)
		cmd.ErrCheck(err)

		cmd.Success("\n%v", "claimed")
	},
}

var filClaimsCmd = &cobra.Command{
	Use:   "claims",
	Short: "List FIL claims for the specified org",
	Long:  `List FIL claims for the specified org.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()

		var opts []hc.ListFilClaimsOption

		ascending, err := c.Flags().GetBool("ascending")
		cmd.ErrCheck(err)
		dev, err := c.Flags().GetBool("dev")
		cmd.ErrCheck(err)

		if ascending {
			opts = append(opts, hc.ListFilClaimsAscending())
		}
		if dev {
			opts = append(opts, hc.ListFilClaimsClaimedByDev())
		}

		// ToDo: Deal with paging.

		claims, err := clients.Hub.ListFilClaims(ctx, opts...)
		cmd.ErrCheck(err)

		if len(claims) > 0 {
			data := make([][]string, len(claims))
			for i, claim := range claims {
				amount := fmt.Sprintf("%.8f", float64(claim.AmountNanoFil)/math.Pow10(9))
				createdAt := claim.CreatedAt.AsTime().Format(time.RFC3339)
				state := sendfilpb.MessageState_name[int32(claim.State)]
				data[i] = []string{amount, state, claim.TxnCid, claim.FailureMessage, createdAt}
			}
			cmd.RenderTable([]string{"amount (fil)", "state", "txn cid", "failure message", "created at"}, data)
		}
	},
}

var filRewardsBalanceCmd = &cobra.Command{
	Use:   "rewards-balance",
	Short: "Show FIL rewards balance",
	Long:  `Show FIL rewards balance.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()

		res, err := clients.Hub.FilRewardsBalance(ctx)
		cmd.ErrCheck(err)

		rewarded := fmt.Sprintf("%.8f", float64(res.RewardedNanoFil)/math.Pow10(9))
		pending := fmt.Sprintf("%.8f", float64(res.ClaimedPendingNanoFil)/math.Pow10(9))
		complete := fmt.Sprintf("%.8f", float64(res.ClaimedCompleteNanoFil)/math.Pow10(9))
		available := fmt.Sprintf("%.8f", float64(res.AvailableNanoFil)/math.Pow10(9))

		cmd.RenderTable(
			[]string{"category", "amount (FIL)"},
			[][]string{
				{"Rewarded", rewarded},
				{"Claimed Pending", pending},
				{"Claimed Complete", complete},
				{"Available", available},
			},
		)
	},
}
