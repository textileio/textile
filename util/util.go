package util

import (
	"crypto/rand"
	"fmt"
	"strings"

	"github.com/gosimple/slug"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	ma "github.com/multiformats/go-multiaddr"
	mbase "github.com/multiformats/go-multibase"
)

func init() {
	slug.MaxLength = 32
	slug.Lowercase = false
}

func ToValidName(str string) (name string, ok bool) {
	name = slug.Make(str)
	if len(name) == 0 {
		ok = false
		return
	}
	return name, true
}

func GenerateRandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func MakeToken(n int) string {
	bytes := GenerateRandomBytes(n)
	encoded, err := mbase.Encode(mbase.Base32, bytes)
	if err != nil {
		panic(err)
	}
	return encoded
}

func MustParseAddr(str string) ma.Multiaddr {
	addr, err := ma.NewMultiaddr(str)
	if err != nil {
		panic(err)
	}
	return addr
}

func NewResolvedPath(s string) (path.Resolved, error) {
	parts := strings.SplitN(s, "/", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("path is not resolvable")
	}
	c, err := cid.Decode(parts[2])
	if err != nil {
		return nil, err
	}
	return path.IpfsPath(c), nil
}

func ParsePath(p path.Path) (resolved path.Resolved, fpath string, err error) {
	parts := strings.SplitN(p.String(), "/", 4)
	if len(parts) < 3 {
		err = fmt.Errorf("path does not contain a resolvable segment")
		return
	}
	c, err := cid.Decode(parts[2])
	if err != nil {
		return
	}
	if len(parts) > 3 {
		fpath = parts[3]
	}
	return path.IpfsPath(c), fpath, nil
}

func ByteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}
