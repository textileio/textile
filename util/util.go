package util

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"fmt"

	"github.com/gosimple/slug"
	ma "github.com/multiformats/go-multiaddr"
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
	return base32.StdEncoding.EncodeToString(bytes)
}

func MakeURLSafeToken(n int) string {
	bytes := GenerateRandomBytes(n)
	return base64.URLEncoding.EncodeToString(bytes)
}

func MustParseAddr(str string) ma.Multiaddr {
	addr, err := ma.NewMultiaddr(str)
	if err != nil {
		panic(err)
	}
	return addr
}
