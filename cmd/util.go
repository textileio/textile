package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/textileio/powergate/exe/cli/cmd"

	"github.com/logrusorgru/aurora"
	"github.com/mitchellh/go-homedir"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/grpc/status"
)

type Flag struct {
	Key      string
	DefValue interface{}
}

const maxSearchHeight = 50

func InitConfig(v *viper.Viper, file, cdir, name string, global bool) func() {
	return func() {
		found := false
		pre := "."
		h := 1
		for h <= maxSearchHeight && !found {
			found = initConfig(v, file, pre, cdir, name, global)
			pre = filepath.Join("../", pre)
			h++
		}
	}
}

func initConfig(v *viper.Viper, file, pre, cdir, name string, global bool) bool {
	if file != "" {
		v.SetConfigFile(file)
	} else {
		v.AddConfigPath(filepath.Join(pre, cdir)) // local config takes priority
		if global {
			home, err := homedir.Dir()
			if err != nil {
				panic(err)
			}
			v.AddConfigPath(filepath.Join(home, cdir))
		}
		v.SetConfigName(name)
	}

	v.SetEnvPrefix("TXTL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()
	if err := v.ReadInConfig(); err != nil && strings.Contains(err.Error(), "Not Found") {
		return false
	}
	return true
}

func WriteConfig(c *cobra.Command, v *viper.Viper, name string) {
	var dir string
	if !c.Flag("dir").Changed {
		home, err := homedir.Dir()
		if err != nil {
			Fatal(err)
		}
		dir = filepath.Join(home, name)
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			Fatal(err)
		}
	} else {
		dir = c.Flag("dir").Value.String()
	}

	filename := filepath.Join(dir, "config.yml")
	if _, err := os.Stat(filename); err == nil {
		cmd.Fatal(fmt.Errorf("%s already exists", filename))
	}
	if err := v.WriteConfigAs(filename); err != nil {
		cmd.Fatal(err)
	}
}

func BindFlags(v *viper.Viper, root *cobra.Command, flags map[string]Flag) error {
	for n, f := range flags {
		if err := v.BindPFlag(f.Key, root.PersistentFlags().Lookup(n)); err != nil {
			return err
		}
		v.SetDefault(f.Key, f.DefValue)
	}
	return nil
}

func ExpandConfigVars(v *viper.Viper, flags map[string]Flag) {
	for _, f := range flags {
		if f.Key != "" {
			if str, ok := v.Get(f.Key).(string); ok {
				v.Set(f.Key, os.ExpandEnv(str))
			}
		}
	}
}

func AddrFromStr(str string) ma.Multiaddr {
	addr, err := ma.NewMultiaddr(str)
	if err != nil {
		Fatal(err)
	}
	return addr
}

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
	fmt.Println(aurora.Sprintf(aurora.Yellow("! "+format), args...))
}

func Success(format string, args ...interface{}) {
	fmt.Println(aurora.Sprintf(aurora.Cyan("> Success! %s"),
		aurora.Sprintf(aurora.BrightBlack(format), args...)))
}

func End(format string, args ...interface{}) {
	Message(format, args...)
	os.Exit(0)
}

func Fatal(err error, args ...interface{}) {
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
	fmt.Println(aurora.Sprintf(aurora.Red("> Error! %s"),
		aurora.Sprintf(aurora.BrightBlack(msg), args...)))
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
	headersColors := make([]tablewriter.Colors, len(data[0]))
	for i := range headersColors {
		headersColors[i] = tablewriter.Colors{tablewriter.FgHiBlackColor}
	}
	table.SetHeaderColor(headersColors...)
	table.AppendBulk(data)
	table.Render()
	fmt.Println()
}
