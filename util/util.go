package util

import (
	"crypto/rand"
	"fmt"

	"github.com/gosimple/slug"
	ma "github.com/multiformats/go-multiaddr"
	mbase "github.com/multiformats/go-multibase"
)

func ToValidName(str string) (name string, err error) {
	name = slug.Make(str)
	if len(name) < 3 {
		err = fmt.Errorf("name must contain at least three URL-safe characters")
		return
	}
	return name, nil
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
