package client

import (
	"context"

	userPb "github.com/textileio/powergate/v2/api/gen/powergate/user/v1"
	"google.golang.org/grpc"
)

// Client provides the client api.
type Client struct {
	powC userPb.UserServiceClient
	conn *grpc.ClientConn
}

// NewClient starts the client.
func NewClient(target string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		powC: userPb.NewUserServiceClient(conn),
		conn: conn,
	}, nil
}

func (c *Client) Addresses(ctx context.Context) (*userPb.AddressesResponse, error) {
	return c.powC.Addresses(ctx, &userPb.AddressesRequest{})
}

func (c *Client) Balance(ctx context.Context, addr string) (*userPb.BalanceResponse, error) {
	req := &userPb.BalanceRequest{
		Address: addr,
	}
	return c.powC.Balance(ctx, req)
}

func (c *Client) SignMessage(ctx context.Context, address string, message []byte) (*userPb.SignMessageResponse, error) {
	req := &userPb.SignMessageRequest{
		Address: address,
		Message: message,
	}
	return c.powC.SignMessage(ctx, req)
}

func (c *Client) VerifyMessage(ctx context.Context, address string, message, signature []byte) (*userPb.VerifyMessageResponse, error) {
	r := &userPb.VerifyMessageRequest{Address: address, Message: message, Signature: signature}
	return c.powC.VerifyMessage(ctx, r)
}

func (c *Client) CidInfo(ctx context.Context, cid string) (*userPb.CidInfoResponse, error) {
	req := &userPb.CidInfoRequest{Cid: cid}
	return c.powC.CidInfo(ctx, req)
}

func (c *Client) StorageDealRecords(ctx context.Context, config *userPb.DealRecordsConfig) (*userPb.StorageDealRecordsResponse, error) {
	req := &userPb.StorageDealRecordsRequest{
		Config: config,
	}
	return c.powC.StorageDealRecords(ctx, req)
}

func (c *Client) RetrievalDealRecords(ctx context.Context, config *userPb.DealRecordsConfig) (*userPb.RetrievalDealRecordsResponse, error) {
	req := &userPb.RetrievalDealRecordsRequest{
		Config: config,
	}
	return c.powC.RetrievalDealRecords(ctx, req)
}

// Close closes the client's grpc connection and cancels any active requests.
func (c *Client) Close() error {
	return c.conn.Close()
}
