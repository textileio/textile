package client

import (
	"context"
	"fmt"
	"time"

	analyticspb "github.com/textileio/textile/v2/api/analyticsd/pb"
	"github.com/textileio/textile/v2/api/filrewardsd/pb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	fsc  pb.FilRewardsServiceClient
	conn *grpc.ClientConn
}

func New(target string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating gRPC client conn: %v", err)
	}

	c := pb.NewFilRewardsServiceClient(conn)

	return &Client{
		fsc:  c,
		conn: conn,
	}, nil
}

func (c *Client) ProcessAnalyticsEvent(ctx context.Context, key string, accountType analyticspb.AccountType, event analyticspb.Event) (*pb.RewardRecord, error) {
	req := &pb.ProcessAnalyticsEventRequest{
		Key:            key,
		AccountType:    accountType,
		AnalyticsEvent: event,
	}
	res, err := c.fsc.ProcessAnalyticsEvent(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("calling process analytics event rpc: %v", err)
	}
	return res.RewardRecord, nil
}

func (c *Client) Claim(ctx context.Context, key string, reward pb.Reward) error {
	req := &pb.ClaimRequest{
		Key:    key,
		Reward: reward,
	}
	_, err := c.fsc.Claim(ctx, req)
	if err != nil {
		return fmt.Errorf("calling claim rpc: %v", err)
	}
	return nil
}

func (c *Client) Get(ctx context.Context, key string, reward pb.Reward) (*pb.RewardRecord, error) {
	req := &pb.GetRequest{
		Key:    key,
		Reward: reward,
	}
	res, err := c.fsc.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("calling get rpc: %v", err)
	}
	return res.RewardRecord, nil
}

type ListOption = func(*pb.ListRequest)

func WithKeyFilter(key string) ListOption {
	return func(req *pb.ListRequest) {
		req.KeyFilter = key
	}
}

func WithRewardFilter(reward pb.Reward) ListOption {
	return func(req *pb.ListRequest) {
		req.RewardFilter = reward
	}
}

func WithClaimedFilter(claimedFilter pb.ClaimedFilter) ListOption {
	return func(req *pb.ListRequest) {
		req.ClaimedFilter = claimedFilter
	}
}

func WithAscending() ListOption {
	return func(req *pb.ListRequest) {
		req.Ascending = true
	}
}

func WithStartAt(time time.Time) ListOption {
	return func(req *pb.ListRequest) {
		req.StartAt = timestamppb.New(time)
	}
}

func WithLimit(limit int64) ListOption {
	return func(req *pb.ListRequest) {
		req.Limit = limit
	}
}

func (c *Client) List(ctx context.Context, opts ...ListOption) ([]*pb.RewardRecord, bool, *time.Time, error) {
	req := &pb.ListRequest{}
	for _, opt := range opts {
		opt(req)
	}
	res, err := c.fsc.List(ctx, req)
	if err != nil {
		return nil, false, nil, fmt.Errorf("calling list rpc: %v", err)
	}
	var t *time.Time
	if res.MoreStartAt != nil {
		ts := res.MoreStartAt.AsTime()
		t = &ts
	}
	return res.GetRewardRecords(), res.More, t, nil
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.conn.Close()
}
