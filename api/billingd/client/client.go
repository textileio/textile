package client

import (
	"context"
	"time"

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

func (c *Client) AddCard(ctx context.Context, customerID, token string) error {
	_, err := c.c.AddCard(ctx, &pb.AddCardRequest{
		CustomerId: customerID,
		Token:      token,
	})
	return err
}

func (c *Client) SetStoredData(ctx context.Context, customerID string, byteSize int64) (units int64, changed bool, err error) {
	res, err := c.c.SetStoredData(ctx, &pb.SetStoredDataRequest{
		CustomerId: customerID,
		ByteSize:   byteSize,
	})
	if err != nil {
		return 0, false, err
	}
	return res.PeriodUnits, res.Changed, nil
}

func (c *Client) IncNetworkEgress(ctx context.Context, customerID string, byteSize int64) (units int64, changed bool, err error) {
	res, err := c.c.IncNetworkEgress(ctx, &pb.IncNetworkEgressRequest{
		CustomerId: customerID,
		ByteSize:   byteSize,
	})
	if err != nil {
		return 0, false, err
	}
	return res.AddedUnits, res.Changed, nil
}

func (c *Client) IncInstanceReads(ctx context.Context, customerID string, count int64) (units int64, changed bool, err error) {
	res, err := c.c.IncInstanceReads(ctx, &pb.IncInstanceReadsRequest{
		CustomerId: customerID,
		Count:      count,
	})
	if err != nil {
		return 0, false, err
	}
	return res.AddedUnits, res.Changed, nil
}

func (c *Client) IncInstanceWrites(ctx context.Context, customerID string, count int64) (units int64, changed bool, err error) {
	res, err := c.c.IncInstanceWrites(ctx, &pb.IncInstanceWritesRequest{
		CustomerId: customerID,
		Count:      count,
	})
	if err != nil {
		return 0, false, err
	}
	return res.AddedUnits, res.Changed, nil
}

type PeriodUsage struct {
	StoredData     PeriodUsageItem
	NetworkEgress  PeriodUsageItem
	InstanceReads  PeriodUsageItem
	InstanceWrites PeriodUsageItem
}

type PeriodUsageItem struct {
	Name   string
	Units  int64
	Period Period
}

type Period struct {
	Start time.Time
	End   time.Time
}

func (c *Client) GetPeriodUsage(ctx context.Context, customerID string) (*PeriodUsage, error) {
	res, err := c.c.GetPeriodUsage(ctx, &pb.GetPeriodUsageRequest{
		CustomerId: customerID,
	})
	if err != nil {
		return nil, err
	}
	return &PeriodUsage{
		StoredData:     usageItemFromPb(res.StoredData),
		NetworkEgress:  usageItemFromPb(res.NetworkEgress),
		InstanceReads:  usageItemFromPb(res.InstanceReads),
		InstanceWrites: usageItemFromPb(res.InstanceWrites),
	}, nil
}

func usageItemFromPb(item *pb.GetPeriodUsageResponse_Item) PeriodUsageItem {
	return PeriodUsageItem{
		Name:  item.Name,
		Units: item.Units,
		Period: Period{
			Start: time.Unix(item.Period.Start, 0),
			End:   time.Unix(item.Period.End, 0),
		},
	}
}

func (c *Client) DeleteCustomer(ctx context.Context, customerID string) error {
	_, err := c.c.DeleteCustomer(ctx, &pb.DeleteCustomerRequest{
		CustomerId: customerID,
	})
	return err
}
