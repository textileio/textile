package gateway

import (
	"testing"

	"github.com/textileio/textile/util"
)

var (
	addrGateway    = util.MustParseAddr("/ip4/127.0.0.1/tcp/8006")
	addrGatewayUrl = "http://127.0.0.1:8006"
	host           = NewGateway(addrGateway, addrGatewayUrl)
)

func TestGateway_Start(t *testing.T) {
	host.Start()
	if len(host.Addr()) == 0 {
		t.Error("get gateway address failed")
		return
	}
}

func TestGateway_Stop(t *testing.T) {
	err := host.Stop()
	if err != nil {
		t.Errorf("stop gateway failed: %s", err)
	}
}
