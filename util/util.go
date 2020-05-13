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
		return nil, fmt.Errorf("invalid path")
	}
	c, err := cid.Decode(parts[2])
	if err != nil {
		return nil, err
	}
	return path.IpfsPath(c), nil
}
