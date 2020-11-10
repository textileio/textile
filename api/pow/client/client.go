package client

import (
	"context"

	pbPow "github.com/textileio/powergate/proto/powergate/v1"
	"google.golang.org/grpc"
)

// Client provides the client api.
type Client struct {
	powC pbPow.PowergateServiceClient
	conn *grpc.ClientConn
}

// NewClient starts the client.
func NewClient(target string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		powC: pbPow.NewPowergateServiceClient(conn),
		conn: conn,
	}, nil
}

func (c *Client) Addresses(ctx context.Context) (*pbPow.AddressesResponse, error) {
	return c.powC.Addresses(ctx, &pbPow.AddressesRequest{})
}

func (c *Client) Balance(ctx context.Context, addr string) (*pbPow.BalanceResponse, error) {
	req := &pbPow.BalanceRequest{
		Address: addr,
	}
	return c.powC.Balance(ctx, req)
}

func (c *Client) CidInfo(ctx context.Context, cids ...string) (*pbPow.CidInfoResponse, error) {
	req := &pbPow.CidInfoRequest{Cids: cids}
	return c.powC.CidInfo(ctx, req)
}

func (c *Client) StorageDealRecords(ctx context.Context, config *pbPow.DealRecordsConfig) (*pbPow.StorageDealRecordsResponse, error) {
	req := &pbPow.StorageDealRecordsRequest{
		Config: config,
	}
	return c.powC.StorageDealRecords(ctx, req)
}

func (c *Client) RetrievalDealRecords(ctx context.Context, config *pbPow.DealRecordsConfig) (*pbPow.RetrievalDealRecordsResponse, error) {
	req := &pbPow.RetrievalDealRecordsRequest{
		Config: config,
	}
	return c.powC.RetrievalDealRecords(ctx, req)
}

// Close closes the client's grpc connection and cancels any active requests.
func (c *Client) Close() error {
	return c.conn.Close()
}
