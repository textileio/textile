package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/logrusorgru/aurora"
	"github.com/olekukonko/tablewriter"
	"github.com/textileio/textile/v2/api/billingd/common"
	"github.com/textileio/textile/v2/api/bucketsd"
	"google.golang.org/grpc/status"
)

func Message(format string, args ...interface{}) {
	if format == "" {
		return
	}
	fmt.Println(aurora.Sprintf(aurora.BrightBlack("> "+format), args...))
}

func Warn(format string, args ...interface{}) {
	if format == "" {
		return
	}
	fmt.Println(aurora.Sprintf(aurora.Yellow("> Warning! %s"),
		aurora.Sprintf(aurora.BrightBlack(format), args...)))
}

func Success(format string, args ...interface{}) {
	fmt.Println(aurora.Sprintf(aurora.Cyan("> Success! %s"),
		aurora.Sprintf(aurora.BrightBlack(format), args...)))
}

func End(format string, args ...interface{}) {
	Message(format, args...)
	os.Exit(0)
}

func Error(err error, args ...interface{}) {
	var msg string
	stat, ok := status.FromError(err)
	if ok {
		msg = stat.Message()
	} else {
		msg = err.Error()
	}
	words := strings.SplitN(msg, " ", 2)
	words[0] = strings.Title(words[0])
	msg = strings.Join(words, " ")

	// @todo: Clean this up somehow?
	if strings.Contains(strings.ToLower(msg), common.ErrExceedsFreeUnits.Error()) ||
		strings.Contains(strings.ToLower(msg), bucketsd.ErrStorageQuotaExhausted.Error()) {
		fmt.Println(aurora.Sprintf(aurora.Red("> Error! %s %s"),
			aurora.Sprintf(aurora.BrightBlack(msg), args...),
			aurora.Sprintf(aurora.BrightBlack("(use `%s` to add a payment method)"),
				aurora.Cyan("hub billing portal"))))
	} else if strings.Contains(strings.ToLower(msg), common.ErrSubscriptionCanceled.Error()) {
		fmt.Println(aurora.Sprintf(aurora.Red("> Error! %s %s"),
			aurora.Sprintf(aurora.BrightBlack(msg), args...),
			aurora.Sprintf(aurora.BrightBlack("(use `%s` to re-enable billing)"),
				aurora.Cyan("hub billing setup"))))
	} else if strings.Contains(strings.ToLower(msg), common.ErrSubscriptionPaymentRequired.Error()) {
		fmt.Println(aurora.Sprintf(aurora.Red("> Error! %s %s"),
			aurora.Sprintf(aurora.BrightBlack(msg), args...),
			aurora.Sprintf(aurora.BrightBlack("(use `%s` to make a payment)"),
				aurora.Cyan("hub billing portal"))))
	} else {
		fmt.Println(aurora.Sprintf(aurora.Red("> Error! %s"),
			aurora.Sprintf(aurora.BrightBlack(msg), args...)))
	}
}

func Fatal(err error, args ...interface{}) {
	Error(err, args...)
	os.Exit(1)
}

func ErrCheck(err error, args ...interface{}) {
	if err != nil {
		Fatal(err, args...)
	}
}

func HandleInterrupt(stop func()) {
	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	fmt.Println("Gracefully stopping... (press Ctrl+C again to force)")
	stop()
	os.Exit(1)
}

func RenderTable(header []string, data [][]string) {
	fmt.Println()
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(header)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(false)
	headersColors := make([]tablewriter.Colors, len(header))
	for i := range headersColors {
		headersColors[i] = tablewriter.Colors{tablewriter.FgHiBlackColor}
	}
	table.SetHeaderColor(headersColors...)
	table.AppendBulk(data)
	table.Render()
	fmt.Println()
}
