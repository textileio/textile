package gateway

import (
	"testing"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-core/broadcast"
)

var (
	bus         = broadcast.NewBroadcaster(0)
	gatewayAddr = parseToAddr("/ip4/127.0.0.1/tcp/9998")
)

func TestGateway_Creation(t *testing.T) {
	Host = &Gateway{Bus: bus}
	Host.Start(gatewayAddr)
}

func TestGateway_Addr(t *testing.T) {
	if len(Host.Addr()) == 0 {
		t.Error("get gateway address failed")
		return
	}
}

func TestGateway_Stop(t *testing.T) {
	err := Host.Stop()
	if err != nil {
		t.Errorf("stop gateway failed: %s", err)
	}
}

func parseToAddr(str string) ma.Multiaddr {
	addr, err := ma.NewMultiaddr(str)
	if err != nil {
		panic(err)
	}
	return addr
}
