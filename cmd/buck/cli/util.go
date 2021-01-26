package cli

import (
	"fmt"
	"os"
	"runtime"

	pb "github.com/cheggaaa/pb/v3"
	"github.com/manifoldco/promptui"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
)

func getConfirm(label string, auto bool) local.ConfirmDiffFunc {
	return func(diff []local.Change) bool {
		if auto {
			return true
		}
		for _, c := range diff {
			cf := local.ChangeColor(c.Type)
			cmd.Message("%s  %s", cf(local.ChangeType(c.Type)), cf(c.Rel))
		}
		prompt := promptui.Prompt{
			Label:     fmt.Sprintf(label, len(diff)),
			IsConfirm: true,
		}
		if _, err := prompt.Run(); err != nil {
			return false
		}
		return true
	}
}

func handleEvents(events chan local.Event) {
	var bar *pb.ProgressBar
	if runtime.GOOS != "windows" {
		bar = pb.New(0)
		bar.Set(pb.Bytes, true)
		tmp := `{{string . "prefix"}}{{counters . }} {{bar . "[" "=" ">" "-" "]"}} {{percent . }} {{etime . }}{{string . "suffix"}}`
		bar.SetTemplate(pb.ProgressBarTemplate(tmp))
	}

	clear := func() {
		if bar != nil {
			_, _ = fmt.Fprintf(os.Stderr, "\033[2K\r")
		}
	}

	for e := range events {
		switch e.Type {
		case local.EventProgress:
			if bar == nil {
				continue
			}
			bar.SetTotal(e.Size)
			bar.SetCurrent(e.Complete)
			if !bar.IsStarted() {
				bar.Start()
			}
			bar.Write()
		case local.EventFileComplete:
			clear()
			_, _ = fmt.Fprintf(os.Stdout,
				"+ %s %s %s\n",
				e.Cid,
				e.Path,
				formatBytes(e.Size, false),
			)
			if bar != nil && bar.IsStarted() {
				bar.Write()
			}
		case local.EventFileRemoved:
			clear()
			_, _ = fmt.Fprintf(os.Stdout, "- %s\n", e.Path)
			if bar != nil && bar.IsStarted() {
				bar.Write()
			}
		}
	}
}

// Copied from https://github.com/cheggaaa/pb/blob/master/v3/util.go
const (
	_KiB = 1024
	_MiB = 1048576
	_GiB = 1073741824
	_TiB = 1099511627776

	_kB = 1e3
	_MB = 1e6
	_GB = 1e9
	_TB = 1e12
)

// Copied from https://github.com/cheggaaa/pb/blob/master/v3/util.go
func formatBytes(i int64, useSIPrefix bool) (result string) {
	if !useSIPrefix {
		switch {
		case i >= _TiB:
			result = fmt.Sprintf("%.02f TiB", float64(i)/_TiB)
		case i >= _GiB:
			result = fmt.Sprintf("%.02f GiB", float64(i)/_GiB)
		case i >= _MiB:
			result = fmt.Sprintf("%.02f MiB", float64(i)/_MiB)
		case i >= _KiB:
			result = fmt.Sprintf("%.02f KiB", float64(i)/_KiB)
		default:
			result = fmt.Sprintf("%d B", i)
		}
	} else {
		switch {
		case i >= _TB:
			result = fmt.Sprintf("%.02f TB", float64(i)/_TB)
		case i >= _GB:
			result = fmt.Sprintf("%.02f GB", float64(i)/_GB)
		case i >= _MB:
			result = fmt.Sprintf("%.02f MB", float64(i)/_MB)
		case i >= _kB:
			result = fmt.Sprintf("%.02f kB", float64(i)/_kB)
		default:
			result = fmt.Sprintf("%d B", i)
		}
	}
	return
}
