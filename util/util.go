package util

import (
	"crypto/rand"
	"strings"

	"github.com/gosimple/slug"
	"github.com/ipfs/go-cid"
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

func Base32Path(p string) string {
	parts := strings.Split(p, "/")
	if len(parts) < 3 {
		return p
	}
	id, err := cid.Decode(parts[2])
	if err != nil {
		return p
	}
	parts[2], err = cid.NewCidV1(id.Type(), id.Hash()).StringOfBase(mbase.Base32)
	if err != nil {
		return p
	}
	return strings.Join(parts, "/")
}
