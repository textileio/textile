package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-merkledag/dagutils"
	"github.com/logrusorgru/aurora"
	"github.com/textileio/textile/buckets/local"
	"github.com/textileio/textile/cmd"
	"github.com/textileio/uiprogress"
)

type change struct {
	Type dagutils.ChangeType
	Path string
	Rel  string
}

func changeType(t dagutils.ChangeType) string {
	switch t {
	case dagutils.Mod:
		return "modified:"
	case dagutils.Add:
		return "new file:"
	case dagutils.Remove:
		return "deleted: "
	default:
		return ""
	}
}

func changeColor(t dagutils.ChangeType) func(arg interface{}) aurora.Value {
	switch t {
	case dagutils.Mod:
		return aurora.Yellow
	case dagutils.Add:
		return aurora.Green
	case dagutils.Remove:
		return aurora.Red
	default:
		return nil
	}
}

func getDiff(buck *local.Bucket, root string) []change {
	cwd, err := os.Getwd()
	if err != nil {
		cmd.Fatal(err)
	}
	rel, err := filepath.Rel(cwd, root)
	if err != nil {
		cmd.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	diff, err := buck.Diff(ctx, rel)
	if err != nil {
		cmd.Fatal(err)
	}
	var all []change
	if len(diff) == 0 {
		return all
	}
	for _, c := range diff {
		r := filepath.Join(rel, c.Path)
		switch c.Type {
		case dagutils.Mod, dagutils.Add:
			names := walkPath(r)
			if len(names) > 0 {
				for _, n := range names {
					p := strings.TrimPrefix(n, rel+"/")
					all = append(all, change{Type: c.Type, Path: p, Rel: n})
				}
			} else {
				all = append(all, change{Type: c.Type, Path: c.Path, Rel: r})
			}
		case dagutils.Remove:
			all = append(all, change{Type: c.Type, Path: c.Path, Rel: r})
		}
	}
	return all
}

func walkPath(pth string) (names []string) {
	if err := filepath.Walk(pth, func(n string, info os.FileInfo, err error) error {
		if err != nil {
			cmd.Fatal(err)
		}
		if !info.IsDir() {
			f := strings.TrimPrefix(n, pth+"/")
			if local.Ignore(n) || strings.HasPrefix(f, configDir) || strings.HasSuffix(f, local.PatchExt) {
				return nil
			}
			names = append(names, n)
		}
		return nil
	}); err != nil {
		cmd.Fatal(err)
	}
	return names
}

func startProgress() {
	uiprogress.Start()
}

func stopProgress() {
	uiprogress.Stop()
}

func addBar(pth string, size int64) *uiprogress.Bar {
	bar := uiprogress.AddBar(int(size)).AppendCompleted()
	pre := "+ " + pth + ":"
	total := formatBytes(size, true)
	setBarWidth(bar, pre, total, 9)
	bar.PrependFunc(func(b *uiprogress.Bar) string {
		c := formatBytes(int64(b.Current()), true)
		return pre + "  " + c + " / " + total
	})
	return bar
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
	if err != nil {
		cmd.Fatal(err)
	}
	if _, err = fmt.Sscan(string(termDim), &h, &w); err != nil {
		cmd.Fatal(err)
	}
	return w, h
}

func finishBar(bar *uiprogress.Bar, pth string, c cid.Cid) {
	bar.Final = "+ " + pth + ": " + c.String()
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
