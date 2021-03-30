package client

import (
	"context"
	"fmt"

	analyticspb "github.com/textileio/textile/v2/api/analyticsd/pb"
	"github.com/textileio/textile/v2/api/filrewardsd/pb"
	"google.golang.org/grpc"
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

type ListRewardsOption = func(*pb.ListRewardsRequest)

func ListRewardsOrgKeyFilter(orgKey string) ListRewardsOption {
	return func(req *pb.ListRewardsRequest) {
		req.OrgKeyFilter = orgKey
	}
}

func ListRewardsDevKeyFilter(devKey string) ListRewardsOption {
	return func(req *pb.ListRewardsRequest) {
		req.DevKeyFilter = devKey
	}
}

func ListRewardsRewardTypeFilter(rewardType pb.RewardType) ListRewardsOption {
	return func(req *pb.ListRewardsRequest) {
		req.RewardTypeFilter = rewardType
	}
}

func ListRewardsAscending() ListRewardsOption {
	return func(req *pb.ListRewardsRequest) {
		req.Ascending = true
	}
}

func ListRewardsPage(page int64) ListRewardsOption {
	return func(req *pb.ListRewardsRequest) {
		req.Page = page
	}
}

func ListRewardsPageSize(pageSize int64) ListRewardsOption {
	return func(req *pb.ListRewardsRequest) {
		req.PageSize = pageSize
	}
}

func (c *Client) ListRewards(ctx context.Context, opts ...ListRewardsOption) ([]*pb.Reward, error) {
	req := &pb.ListRewardsRequest{}
	for _, opt := range opts {
		opt(req)
	}
	res, err := c.fsc.ListRewards(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("calling list rewards rpc: %v", err)
	}
	return res.Rewards, nil
}

func (c *Client) Claim(ctx context.Context, orgKey, claimedBy string, amountNanoFil int64) (*pb.Claim, error) {
	req := &pb.ClaimRequest{
		OrgKey:        orgKey,
		ClaimedBy:     claimedBy,
		AmountNanoFil: amountNanoFil,
	}
	res, err := c.fsc.Claim(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("calling claim rpc: %v", err)
	}
	return res.Claim, nil
}

type ListClaimsOption = func(*pb.ListClaimsRequest)

func ListClaimsOrgKeyFilter(orgKey string) ListClaimsOption {
	return func(req *pb.ListClaimsRequest) {
		req.OrgKeyFilter = orgKey
	}
}

func ListClaimsClaimedByFilter(claimedBy string) ListClaimsOption {
	return func(req *pb.ListClaimsRequest) {
		req.ClaimedByFilter = claimedBy
	}
}

func ListClaimsAscending() ListClaimsOption {
	return func(req *pb.ListClaimsRequest) {
		req.Ascending = true
	}
}

func ListClaimsPage(page int64) ListClaimsOption {
	return func(req *pb.ListClaimsRequest) {
		req.Page = page
	}
}

func ListClaimsPageSize(pageSize int64) ListClaimsOption {
	return func(req *pb.ListClaimsRequest) {
		req.PageSize = pageSize
	}
}

func (c *Client) ListClaims(ctx context.Context, opts ...ListClaimsOption) ([]*pb.Claim, error) {
	req := &pb.ListClaimsRequest{}
	for _, opt := range opts {
		opt(req)
	}
	res, err := c.fsc.ListClaims(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("calling list claims rpc: %v", err)
	}
	return res.Claims, nil
}

func (c *Client) Balance(ctx context.Context, orgKey string) (*pb.BalanceResponse, error) {
	req := &pb.BalanceRequest{OrgKey: orgKey}
	res, err := c.fsc.Balance(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("calling balance rpc: %v", err)
	}
	return res, nil
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.conn.Close()
}
