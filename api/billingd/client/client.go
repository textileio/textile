package client

import (
	"context"

	stripe "github.com/stripe/stripe-go/v72"
	"github.com/textileio/go-threads/core/thread"
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

func (c *Client) CreateCustomer(ctx context.Context, key thread.PubKey, opts ...Option) (string, error) {
	args := &options{}
	for _, opt := range opts {
		opt(args)
	}
	var parentKey string
	if args.parentKey != nil {
		parentKey = args.parentKey.String()
	}
	res, err := c.c.CreateCustomer(ctx, &pb.CreateCustomerRequest{
		Key:         key.String(),
		ParentKey:   parentKey,
		Email:       args.email,
		AccountType: int64(args.accountType),
	})
	if err != nil {
		return "", err
	}
	return res.CustomerId, nil
}

func (c *Client) GetCustomer(ctx context.Context, key thread.PubKey) (*pb.GetCustomerResponse, error) {
	return c.c.GetCustomer(ctx, &pb.GetCustomerRequest{
		Key: key.String(),
	})
}

func (c *Client) GetCustomerSession(ctx context.Context, key thread.PubKey) (*pb.GetCustomerSessionResponse, error) {
	return c.c.GetCustomerSession(ctx, &pb.GetCustomerSessionRequest{
		Key: key.String(),
	})
}

func (c *Client) ListDependentCustomers(ctx context.Context, key thread.PubKey, opts ...ListOption) (
	*pb.ListDependentCustomersResponse, error) {
	args := &listOptions{}
	for _, opt := range opts {
		opt(args)
	}
	return c.c.ListDependentCustomers(ctx, &pb.ListDependentCustomersRequest{
		Key:    key.String(),
		Offset: args.offset,
		Limit:  args.limit,
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
		InvoicePeriod: &pb.Period{
			UnixStart: periodStart,
			UnixEnd:   periodEnd,
		},
	})
	return err
}

func (c *Client) RecreateCustomerSubscription(ctx context.Context, key thread.PubKey) error {
	_, err := c.c.RecreateCustomerSubscription(ctx, &pb.RecreateCustomerSubscriptionRequest{
		Key: key.String(),
	})
	return err
}

func (c *Client) DeleteCustomer(ctx context.Context, key thread.PubKey) error {
	_, err := c.c.DeleteCustomer(ctx, &pb.DeleteCustomerRequest{
		Key: key.String(),
	})
	return err
}

func (c *Client) IncCustomerUsage(
	ctx context.Context,
	key thread.PubKey,
	productUsage map[string]int64,
) (*pb.IncCustomerUsageResponse, error) {
	return c.c.IncCustomerUsage(ctx, &pb.IncCustomerUsageRequest{
		Key:          key.String(),
		ProductUsage: productUsage,
	})
}
