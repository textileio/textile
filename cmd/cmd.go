package cmd

import (
	"strconv"

	"github.com/spf13/pflag"
)

func GetStringFlag(f *pflag.Flag) string {
	if f == nil {
		return ""
	}
	if f.Changed {
		return f.Value.String()
	} else {
		return f.DefValue
	}
}

func GetBoolFlag(f *pflag.Flag) bool {
	if f == nil {
		return false
	}
	var str string
	if f.Changed {
		str = f.Value.String()
	} else {
		str = f.DefValue
	}
	val, err := strconv.ParseBool(str)
	if err != nil {
		return false
	}
	return val
}
