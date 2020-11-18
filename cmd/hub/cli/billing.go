package cli

import (
	"context"
	"fmt"
	"math"
	"sort"
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
		var rows [][]string
		for _, usage := range cus.DailyUsage {
			rows = append(rows, getUsageRow(usage, cus.InvoicePeriod))
		}
		cmd.RenderTable(
			[]string{"", "usage", "free quota", "daily cost", "start", "end"},
			rows,
		)
		if !cus.Billable && cus.GracePeriodEnd > 0 {
			ends := time.Unix(cus.GracePeriodEnd, 0).Format("02-Jan-06 15:04 -0700")
			cmd.Warn("Free quota grace period ends: %d", aurora.Bold(ends))
		}
	},
}

func getUsageRow(usage *pb.Usage, period *pb.Period) []string {
	return []string{
		usage.Description,
		strconv.Itoa(int(usage.Total)),
		fmt.Sprintf(
			"%s (%d%%)",
			strconv.Itoa(int(usage.Free)),
			int(math.Round(100*float64(usage.Free)/float64(usage.Total+usage.Free)))),
		fmt.Sprintf("$%.4f", usage.Cost),
		time.Unix(period.UnixStart, 0).Format("02-Jan-06"),
		time.Unix(period.UnixEnd, 0).Format("02-Jan-06"),
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
			keys := make([]string, 0, len(u.DailyUsage))
			for k := range u.DailyUsage {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var usage string
			for i, k := range keys {
				usage += k + "=" + strconv.Itoa(int(u.DailyUsage[k].Total))
				if i != len(keys)-1 {
					usage += " "
				}
			}
			data[i] = []string{u.Key, usage}
		}
		cmd.RenderTable([]string{"user", "usage"}, data)

		cmd.Message("Next offset: %d", aurora.White(list.NextOffset).Bold())
		cmd.Message("Press 'Enter' to show more...")
		_, _ = fmt.Scanln()
		listUsers(limit, list.NextOffset)
	} else {
		cmd.Message("Found %d users", aurora.White(0).Bold())
	}
}
