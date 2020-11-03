package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/textileio/textile/v2/api/bucketsd"

	"github.com/logrusorgru/aurora"
	"github.com/olekukonko/tablewriter"
	"github.com/textileio/textile/v2/api/billingd/common"
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
	if strings.Contains(msg, common.ErrExceedsFreeUnits.Error()) ||
		strings.Contains(strings.ToLower(msg), bucketsd.ErrStorageQuotaExhausted.Error()) {
		fmt.Println(aurora.Sprintf(aurora.Red("> Error! %s %s"),
			aurora.Sprintf(aurora.BrightBlack(msg), args...),
			aurora.Sprintf(aurora.BrightBlack("(Use `%s` to add a payment method)"),
				aurora.Cyan("hub billing portal"))))
	} else {
		fmt.Println(msg)
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
