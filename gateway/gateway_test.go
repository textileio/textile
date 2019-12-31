package gateway

import (
	"fmt"
	"testing"

	"github.com/phayes/freeport"
	"github.com/textileio/textile/util"
)

func TestNewGateway(t *testing.T) {
	port, err := freeport.GetFreePort()
	if err != nil {
		t.Fatal(err)
	}
	addr := util.MustParseAddr(fmt.Sprintf("/ip4/127.0.0.1/tcp/%d", port))
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	gateway := NewGateway(addr, url, nil)

	t.Run("test start", func(t *testing.T) {
		gateway.Start()
		if len(gateway.Addr()) == 0 {
			t.Error("get gateway address failed")
			return
		}
	})

	t.Run("test stop", func(t *testing.T) {
		if err := gateway.Stop(); err != nil {
			t.Errorf("stop gateway failed: %s", err)
		}
	})
}
