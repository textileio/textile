package client

import (
	"context"

	ffsRpc "github.com/textileio/powergate/ffs/rpc"
	healthRpc "github.com/textileio/powergate/health/rpc"
	netRpc "github.com/textileio/powergate/net/rpc"
	walletRpc "github.com/textileio/powergate/wallet/rpc"
	"google.golang.org/grpc"
)

// Client provides the client api.
type Client struct {
	healthC healthRpc.RPCServiceClient
	netC    netRpc.RPCServiceClient
	ffsC    ffsRpc.RPCServiceClient
	walletC walletRpc.RPCServiceClient
	conn    *grpc.ClientConn
}

// NewClient starts the client.
func NewClient(target string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		healthC: healthRpc.NewRPCServiceClient(conn),
		netC:    netRpc.NewRPCServiceClient(conn),
		ffsC:    ffsRpc.NewRPCServiceClient(conn),
		walletC: walletRpc.NewRPCServiceClient(conn),
		conn:    conn,
	}, nil
}

func (c *Client) Health(ctx context.Context) (*healthRpc.CheckResponse, error) {
	return c.healthC.Check(ctx, &healthRpc.CheckRequest{})
}

func (c *Client) Peers(ctx context.Context) (*netRpc.PeersResponse, error) {
	return c.netC.Peers(ctx, &netRpc.PeersRequest{})
}

func (c *Client) FindPeer(ctx context.Context, peerID string) (*netRpc.FindPeerResponse, error) {
	req := &netRpc.FindPeerRequest{
		PeerId: peerID,
	}
	return c.netC.FindPeer(ctx, req)
}

func (c *Client) Connectedness(ctx context.Context, peerID string) (*netRpc.ConnectednessResponse, error) {
	req := &netRpc.ConnectednessRequest{
		PeerId: peerID,
	}
	return c.netC.Connectedness(ctx, req)
}

func (c *Client) Addrs(ctx context.Context) (*ffsRpc.AddrsResponse, error) {
	return c.ffsC.Addrs(ctx, &ffsRpc.AddrsRequest{})
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
