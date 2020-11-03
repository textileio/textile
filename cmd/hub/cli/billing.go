package cli

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/textileio/textile/v2/api/billingd/pb"

	"github.com/logrusorgru/aurora"
	"github.com/spf13/cobra"
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

var billingStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show usage and billing info",
	Long:  `Shows usage and billing information.`,
	Args:  cobra.ExactArgs(0),
	Run: func(c *cobra.Command, args []string) {
		ctx, cancel := context.WithTimeout(Auth(context.Background()), cmd.Timeout)
		defer cancel()
		info, err := clients.Hub.GetBillingInfo(ctx)
		cmd.ErrCheck(err)
		cus := info.Customer

		cmd.Message("Subscription status: %s", aurora.White(cus.Status).Bold())
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
