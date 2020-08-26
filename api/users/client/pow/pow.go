package pow

import (
	"context"

	ffsRpc "github.com/textileio/powergate/ffs/rpc"
	walletRpc "github.com/textileio/powergate/wallet/rpc"
	"google.golang.org/grpc"
)

// Client provides the client api.
type Client struct {
	ffsC    ffsRpc.RPCServiceClient
	walletC walletRpc.RPCServiceClient
	conn    *grpc.ClientConn
}

// NewClient starts the client.
func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{
		ffsC:    ffsRpc.NewRPCServiceClient(conn),
		walletC: walletRpc.NewRPCServiceClient(conn),
		conn:    conn,
	}
}

func (c *Client) Addrs(ctx context.Context) (*ffsRpc.AddrsResponse, error) {
	return c.ffsC.Addrs(ctx, &ffsRpc.AddrsRequest{})
}

func (c *Client) NewAddr(ctx context.Context, name, addrType string, makeDefault bool) (*ffsRpc.NewAddrResponse, error) {
	req := &ffsRpc.NewAddrRequest{
		Name:        name,
		AddressType: addrType,
		MakeDefault: makeDefault,
	}
	return c.ffsC.NewAddr(ctx, req)
}

func (c *Client) SendFil(ctx context.Context, from, to string, amt int64) error {
	req := &ffsRpc.SendFilRequest{
		Amount: amt,
		From:   from,
		To:     to,
	}
	_, err := c.ffsC.SendFil(ctx, req)
	return err
}

func (c *Client) Info(ctx context.Context) (*ffsRpc.InfoResponse, error) {
	return c.ffsC.Info(ctx, &ffsRpc.InfoRequest{})
}

func (c *Client) Show(ctx context.Context, cid string) (*ffsRpc.ShowResponse, error) {
	req := &ffsRpc.ShowRequest{
		Cid: cid,
	}
	return c.ffsC.Show(ctx, req)
}

func (c *Client) ShowAll(ctx context.Context) (*ffsRpc.ShowAllResponse, error) {
	return c.ffsC.ShowAll(ctx, &ffsRpc.ShowAllRequest{})
}

func (c *Client) ListStorageDealRecords(ctx context.Context, config *ffsRpc.ListDealRecordsConfig) (*ffsRpc.ListStorageDealRecordsResponse, error) {
	req := &ffsRpc.ListStorageDealRecordsRequest{
		Config: config,
	}
	return c.ffsC.ListStorageDealRecords(ctx, req)
}

func (c *Client) ListRetrievalDealRecords(ctx context.Context, config *ffsRpc.ListDealRecordsConfig) (*ffsRpc.ListRetrievalDealRecordsResponse, error) {
	req := &ffsRpc.ListRetrievalDealRecordsRequest{
		Config: config,
	}
	return c.ffsC.ListRetrievalDealRecords(ctx, req)
}

func (c *Client) Balance(ctx context.Context, addr string) (*walletRpc.BalanceResponse, error) {
	req := &walletRpc.BalanceRequest{
		Address: addr,
	}
	return c.walletC.Balance(ctx, req)
}

// Close closes the client's grpc connection and cancels any active requests.
func (c *Client) Close() error {
	return c.conn.Close()
}
