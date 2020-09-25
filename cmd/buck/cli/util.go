package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ipfs/go-cid"
	"github.com/manifoldco/promptui"
	"github.com/textileio/textile/v2/buckets/local"
	"github.com/textileio/textile/v2/cmd"
	"github.com/textileio/uiprogress"
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

func handleProgressBars(p *uiprogress.Progress, events chan local.PathEvent) {
	bars := make(map[string]*uiprogress.Bar)
	for e := range events {
		switch e.Type {
		case local.FileStart:
			bars[e.Path] = addBar(p, e.Path, e.Size)
		case local.FileProgress, local.FileComplete:
			bar, ok := bars[e.Path]
			if ok {
				_ = bar.Set(int(e.Progress))
				if e.Type == local.FileComplete {
					finishBar(p, bar, e.Path, e.Cid, false)
					delete(bars, e.Path)
				}
			}
		case local.FileRemoved:
			bar := p.AddBar(int(e.Size))
			finishBar(p, bar, e.Path, e.Cid, true)
		}
	}
}

func addBar(p *uiprogress.Progress, pth string, size int64) *uiprogress.Bar {
	bar := p.AddBar(int(size)).AppendCompleted()
	pre := "+ " + pth + ":"
	total := formatBytes(size, true)
	setBarWidth(bar, pre, total, 9)
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		c := formatBytes(int64(b.Current()), true)
		return pre + "  " + c + " / " + total
	})
	return bar
}

func finishBar(p *uiprogress.Progress, bar *uiprogress.Bar, pth string, c cid.Cid, removal bool) {
	if removal {
		bar.Final = "- " + pth
	} else {
		bar.Final = "+ " + pth + ": " + c.String()
	}
	p.Print()
}

func setBarWidth(bar *uiprogress.Bar, pre, size string, of int) {
	tw, _ := getTermDim()
	w := tw - len(pre) - (2*len(size) + 3) - of // Make space for overflow chars
	if w > 0 {
		bar.Width = w
	} else {
		bar.Width = 10
	}
}

func getTermDim() (w, h int) {
	c := exec.Command("stty", "size")
	c.Stdin = os.Stdin
	termDim, err := c.Output()
	cmd.ErrCheck(err)
	_, err = fmt.Sscan(string(termDim), &h, &w)
	cmd.ErrCheck(err)
	return w, h
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
