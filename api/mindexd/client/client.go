package client

import (
	"context"
	"fmt"

	"github.com/textileio/textile/v2/api/mindexd/pb"
	"google.golang.org/grpc"
)

// Client provides the client api.
type Client struct {
	c    pb.APIServiceClient
	conn *grpc.ClientConn
}

// NewClient starts the client.
func NewClient(target string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		c:    pb.NewAPIServiceClient(conn),
		conn: conn,
	}, nil
}

// Close closes the client's grpc connection and cancels any active requests.
func (c *Client) Close() error {
	return c.conn.Close()
}

// GetMinerInfo returns miner's information from the index.
func (c *Client) GetMinerInfo(ctx context.Context, minerAddr string) (*pb.GetMinerInfoResponse, error) {
	return c.c.GetMinerInfo(ctx, &pb.GetMinerInfoRequest{
		MinerAddress: minerAddr,
	})
}

// CalcualteDealPrice calcualtes the deal price for the provided data for a list of miners.
func (c *Client) CalculateDealPrice(ctx context.Context, minersAddr []string, dataSizeBytes int64, durationDays int64) (*pb.CalculateDealPriceResponse, error) {
	if len(minersAddr) == 0 {
		return nil, fmt.Errorf("miner's list can't be empty")
	}
	return c.c.CalculateDealPrice(ctx, &pb.CalculateDealPriceRequest{
		DataSizeBytes:  dataSizeBytes,
		DurationDays:   durationDays,
		MinerAddresses: minersAddr,
	})
}

func (c *Client) QueryIndex(ctx context.Context, params *pb.QueryIndexRequest) (*pb.QueryIndexResponse, error) {
	return c.c.QueryIndex(ctx, params)
}
