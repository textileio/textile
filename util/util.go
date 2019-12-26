package util

import (
	ma "github.com/multiformats/go-multiaddr"
)

func MustParseAddr(str string) ma.Multiaddr {
	addr, err := ma.NewMultiaddr(str)
	if err != nil {
		panic(err)
	}
	return addr
}
