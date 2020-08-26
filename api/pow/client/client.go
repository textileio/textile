package client

import (
	"context"

	healthRpc "github.com/textileio/powergate/health/rpc"
	netRpc "github.com/textileio/powergate/net/rpc"
	"google.golang.org/grpc"
)

// Client provides the client api.
type Client struct {
	healthC healthRpc.RPCServiceClient
	netC    netRpc.RPCServiceClient
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
		conn:    conn,
	}, nil
}

func (c *Client) HealthCheck(ctx context.Context) (*healthRpc.CheckResponse, error) {
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

// Close closes the client's grpc connection and cancels any active requests.
func (c *Client) Close() error {
	return c.conn.Close()
}
