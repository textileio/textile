package cli

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
	"github.com/textileio/textile/v2/api/billingd/pb"
	hub "github.com/textileio/textile/v2/api/hubd/client"
	users "github.com/textileio/textile/v2/api/usersd/client"
	"github.com/textileio/textile/v2/cmd"
)

var billingCmd = &cobra.Command{
	Use:   "billing",
	Short: "Billing management",
	Long:  `Manages your billing preferences.`,
	Args:  cobra.ExactArgs(0),
}

var billingSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup usage billing",
	Long:  `Sets up metered usage billing.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		err := clients.Hub.SetupBilling(ctx)
		cmd.ErrCheck(err)

		cmd.Success("You have setup metered usage billing. "+
			"Use `%s` to manage your subscripton and payment methods.", aurora.Cyan("hub billing portal"))
	},
}

var billingPortalCmd = &cobra.Command{
	Use:   "portal",
	Short: "Open billing web portal",
	Long:  `Opens a web portal for managing billing preferences.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		session, err := clients.Hub.GetBillingSession(ctx)
		cmd.ErrCheck(err)

		cmd.Message("Please visit the following URL to manage usage billing:")
		cmd.Message("%s", aurora.White(session.Url).Bold())
	},
}

var billingUsageCmd = &cobra.Command{
	Use:   "usage",
	Short: "Show usage and billing info",
	Long: `Shows usage and billing information.

Use the --user flag to get usage for a dependent user.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(c *cobra.Command, args []string) {
		user, err := c.Flags().GetString("user")
		cmd.ErrCheck(err)

		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		info, err := clients.Users.GetUsage(ctx, users.WithPubKey(user))
		cmd.ErrCheck(err)
		cus := info.Usage

		cmd.Message("Account status: %s", aurora.White(cus.AccountStatus).Bold())
		if cus.ParentKey == "" {
			balance := float64(cus.Balance) / 100
			cmd.Message("Account balance: %s%.2f", aurora.White("$").Bold(), aurora.White(balance).Bold())
		}
		cmd.Message("Subscription status: %s", aurora.White(cus.SubscriptionStatus).Bold())
		if cus.Dependents > 0 {
			cmd.Message("Contributing users: %d", aurora.White(cus.Dependents).Bold())
		}
		cmd.RenderTable(
			[]string{"", "usage", "free quota", "start", "end"},
			[][]string{
				getUsageRow("Stored data (bytes)", cus.StoredData, cus.Period),
				getUsageRow("Network egress (bytes)", cus.NetworkEgress, cus.Period),
				getUsageRow("ThreadDB reads", cus.InstanceReads, cus.Period),
				getUsageRow("ThreadDB writes", cus.InstanceWrites, cus.Period),
			},
		)
	},
}

func getUsageRow(name string, usage *pb.Usage, period *pb.Period) []string {
	return []string{
		name,
		strconv.Itoa(int(usage.Total)),
		fmt.Sprintf(
			"%s (%d%%)",
			strconv.Itoa(int(usage.Free)),
			int(math.Round(100*float64(usage.Free)/float64(usage.Total+usage.Free)))),
		time.Unix(period.Start, 0).Format("02-Jan-06"),
		time.Unix(period.End, 0).Format("02-Jan-06"),
	}
}

var billingUsersCmd = &cobra.Command{
	Use:   "users",
	Short: "list contributing users",
	Long:  `Lists users contributing to billing usage.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		limit, err := c.Flags().GetInt64("limit")
		cmd.ErrCheck(err)
		offset, err := c.Flags().GetInt64("offset")
		cmd.ErrCheck(err)
		listUsers(limit, offset)
	},
}

func listUsers(limit, offset int64) {
	ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
	defer cancel()
	list, err := clients.Hub.ListBillingUsers(ctx, hub.WithLimit(limit), hub.WithOffset(offset))
	cmd.ErrCheck(err)

	if len(list.Users) > 0 {
		data := make([][]string, len(list.Users))
		for i, u := range list.Users {
			data[i] = []string{
				u.Key,
				strconv.Itoa(int(u.StoredData.Total)),
				strconv.Itoa(int(u.NetworkEgress.Total)),
				strconv.Itoa(int(u.InstanceReads.Total)),
				strconv.Itoa(int(u.InstanceWrites.Total)),
			}
		}
		cmd.RenderTable([]string{"user", "data (bytes)", "egress (bytes)", "reads", "writes"}, data)

		cmd.Message("Next offset: %d", aurora.White(list.NextOffset).Bold())
		cmd.Message("Press 'Enter' to show more...")
		_, _ = fmt.Scanln()
		listUsers(limit, list.NextOffset)
	}
}
