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

func (c *Client) ProcessAnalyticsEvent(ctx context.Context, orgKey, devKey string, event analyticspb.Event) (*pb.Reward, error) {
	req := &pb.ProcessAnalyticsEventRequest{
		OrgKey:         orgKey,
		DevKey:         devKey,
		AnalyticsEvent: event,
	}
	res, err := c.fsc.ProcessAnalyticsEvent(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("calling process analytics event rpc: %v", err)
	}
	return res.Reward, nil
}

// func (c *Client) Claim(ctx context.Context, key string, reward pb.Reward) error {
// 	req := &pb.ClaimRequest{
// 		Key:    key,
// 		Reward: reward,
// 	}
// 	_, err := c.fsc.Claim(ctx, req)
// 	if err != nil {
// 		return fmt.Errorf("calling claim rpc: %v", err)
// 	}
// 	return nil
// }

type ListOption = func(*pb.ListRewardsRequest)

func WithOrgKeyFilter(orgKey string) ListOption {
	return func(req *pb.ListRewardsRequest) {
		req.OrgKeyFilter = orgKey
	}
}

func WithDevKeyFilter(devKey string) ListOption {
	return func(req *pb.ListRewardsRequest) {
		req.DevKeyFilter = devKey
	}
}

func WithRewardTypeFilter(rewardType pb.RewardType) ListOption {
	return func(req *pb.ListRewardsRequest) {
		req.RewardTypeFilter = rewardType
	}
}

func WithAscending() ListOption {
	return func(req *pb.ListRewardsRequest) {
		req.Ascending = true
	}
}

func WithStartAt(time time.Time) ListOption {
	return func(req *pb.ListRewardsRequest) {
		req.StartAt = timestamppb.New(time)
	}
}

func WithLimit(limit int64) ListOption {
	return func(req *pb.ListRewardsRequest) {
		req.Limit = limit
	}
}

func (c *Client) ListRewards(ctx context.Context, opts ...ListOption) ([]*pb.Reward, bool, *time.Time, error) {
	req := &pb.ListRewardsRequest{}
	for _, opt := range opts {
		opt(req)
	}
	res, err := c.fsc.ListRewards(ctx, req)
	if err != nil {
		return nil, false, nil, fmt.Errorf("calling list rpc: %v", err)
	}
	var t *time.Time
	if res.MoreStartAt != nil {
		ts := res.MoreStartAt.AsTime()
		t = &ts
	}
	return res.Rewards, res.More, t, nil
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.conn.Close()
}
