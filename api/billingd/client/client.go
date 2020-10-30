package client

import (
	"context"

	stripe "github.com/stripe/stripe-go/v72"
	pb "github.com/textileio/textile/v2/api/billingd/pb"
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

func (c *Client) CheckHealth(ctx context.Context) error {
	_, err := c.c.CheckHealth(ctx, &pb.CheckHealthRequest{})
	return err
}

func (c *Client) CreateCustomer(ctx context.Context, email string) (string, error) {
	res, err := c.c.CreateCustomer(ctx, &pb.CreateCustomerRequest{
		Email: email,
	})
	if err != nil {
		return "", err
	}
	return res.CustomerId, nil
}

func (c *Client) GetCustomer(ctx context.Context, customerID string) (*pb.GetCustomerResponse, error) {
	return c.c.GetCustomer(ctx, &pb.GetCustomerRequest{
		CustomerId: customerID,
	})
}

func (c *Client) GetCustomerSession(ctx context.Context, customerID string) (*pb.GetCustomerSessionResponse, error) {
	return c.c.GetCustomerSession(ctx, &pb.GetCustomerSessionRequest{
		CustomerId: customerID,
	})
}

func (c *Client) UpdateCustomer(
	ctx context.Context,
	customerID string,
	balance int64,
	billable,
	delinquent bool,
) error {
	_, err := c.c.UpdateCustomer(ctx, &pb.UpdateCustomerRequest{
		CustomerId: customerID,
		Balance:    balance,
		Billable:   billable,
		Delinquent: delinquent,
	})
	return err
}

func (c *Client) UpdateCustomerSubscription(
	ctx context.Context,
	customerID string,
	status stripe.SubscriptionStatus,
	periodStart, periodEnd int64,
) error {
	_, err := c.c.UpdateCustomerSubscription(ctx, &pb.UpdateCustomerSubscriptionRequest{
		CustomerId: customerID,
		Status:     string(status),
		Period: &pb.UsagePeriod{
			Start: periodStart,
			End:   periodEnd,
		},
	})
	return err
}

func (c *Client) RecreateCustomerSubscription(ctx context.Context, customerID string) error {
	_, err := c.c.RecreateCustomerSubscription(ctx, &pb.RecreateCustomerSubscriptionRequest{
		CustomerId: customerID,
	})
	return err
}

func (c *Client) DeleteCustomer(ctx context.Context, customerID string) error {
	_, err := c.c.DeleteCustomer(ctx, &pb.DeleteCustomerRequest{
		CustomerId: customerID,
	})
	return err
}

func (c *Client) IncStoredData(
	ctx context.Context,
	customerID string,
	incSize int64,
) (*pb.IncStoredDataResponse, error) {
	return c.c.IncStoredData(ctx, &pb.IncStoredDataRequest{
		CustomerId: customerID,
		IncSize:    incSize,
	})
}

func (c *Client) IncNetworkEgress(
	ctx context.Context,
	customerID string,
	incSize int64,
) (*pb.IncNetworkEgressResponse, error) {
	return c.c.IncNetworkEgress(ctx, &pb.IncNetworkEgressRequest{
		CustomerId: customerID,
		IncSize:    incSize,
	})
}

func (c *Client) IncInstanceReads(
	ctx context.Context,
	customerID string,
	incCount int64,
) (*pb.IncInstanceReadsResponse, error) {
	return c.c.IncInstanceReads(ctx, &pb.IncInstanceReadsRequest{
		CustomerId: customerID,
		IncCount:   incCount,
	})
}

func (c *Client) IncInstanceWrites(
	ctx context.Context,
	customerID string,
	incCount int64,
) (*pb.IncInstanceWritesResponse, error) {
	return c.c.IncInstanceWrites(ctx, &pb.IncInstanceWritesRequest{
		CustomerId: customerID,
		IncCount:   incCount,
	})
}
