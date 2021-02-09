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
	uc "github.com/textileio/textile/v2/api/usersd/client"
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
	Short: "Get information about the current storate state of a cid",
	Long:  `Get information about the current storate state of a cid`,
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

		var opts []uc.ListFilRewardsOption

		ascending, err := c.Flags().GetBool("ascending")
		cmd.ErrCheck(err)
		dev, err := c.Flags().GetBool("dev")
		cmd.ErrCheck(err)

		if ascending {
			opts = append(opts, uc.ListFilRewardsAscending())
		}
		if dev {
			opts = append(opts, uc.ListFilRewardsUnlockedByDev())
		}

		rewards, _, _, err := clients.Users.ListFilRewards(ctx, opts...)
		cmd.ErrCheck(err)

		if len(rewards) > 0 {
			data := make([][]string, len(rewards))
			for i, reward := range rewards {
				attofil := reward.BaseAttoFilReward * reward.Factor
				fil := fmt.Sprintf("%f", float64(attofil)/math.Pow10(18)) // ToDo: What is the right conversion?
				createdAt := reward.CreatedAt.AsTime().Format(time.RFC3339)
				t := filrewardspb.RewardType_name[int32(reward.Type)]
				data[i] = []string{t, fil, reward.OrgKey, reward.DevKey, createdAt}
			}
			cmd.RenderTable([]string{"type", "amount (fil)", "org", "dev", "created at"}, data)
		}
	},
}

var filClaimCmd = &cobra.Command{
	Use:   "claim",
	Short: "Claim unlocked FIL rewards to the specified org",
	Long:  `Claim unlocked FIL rewards to the specified org.`,
	Args:  cobra.ExactArgs(1),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()

		fil, err := strconv.ParseFloat(args[0], 64)
		cmd.ErrCheck(err)
		attofil := int32(fil * math.Pow10(18))

		err = clients.Users.ClaimFil(ctx, attofil)
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

		var opts []uc.ListFilClaimsOption

		ascending, err := c.Flags().GetBool("ascending")
		cmd.ErrCheck(err)
		dev, err := c.Flags().GetBool("dev")
		cmd.ErrCheck(err)

		if ascending {
			opts = append(opts, uc.ListFilClaimsAscending())
		}
		if dev {
			opts = append(opts, uc.ListFilClaimsClaimedByDev())
		}

		claims, _, _, err := clients.Users.ListFilClaims(ctx, opts...)
		cmd.ErrCheck(err)

		if len(claims) > 0 {
			data := make([][]string, len(claims))
			for i, claim := range claims {
				amount := fmt.Sprintf("%f", float64(claim.Amount)/math.Pow10(18)) // ToDo: What is the right conversion?
				createdAt := claim.CreatedAt.AsTime().Format(time.RFC3339)
				state := filrewardspb.ClaimState_name[int32(claim.State)]
				data[i] = []string{amount, claim.OrgKey, claim.ClaimedBy, state, claim.TxnCid, claim.FailureMessage, createdAt}
			}
			cmd.RenderTable([]string{"amount (fil)", "org", "claimed by", "state", "txn cid", "failure message", "created at"}, data)
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

		res, err := clients.Users.FilRewardsBalance(ctx)
		cmd.ErrCheck(err)

		rewarded := fmt.Sprintf("%f", float64(res.Rewarded)/math.Pow10(18))
		pending := fmt.Sprintf("%f", float64(res.Pending)/math.Pow10(18))
		claimed := fmt.Sprintf("%f", float64(res.Claimed)/math.Pow10(18))
		available := fmt.Sprintf("%f", float64(res.Available)/math.Pow10(18))

		cmd.RenderTable(
			[]string{"category"},
			[][]string{
				{"rewarded", rewarded},
				{"pending", pending},
				{"claimed", claimed},
				{"available", available},
			},
		)
	},
}
