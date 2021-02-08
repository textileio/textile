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

type ProcessAnalyticsEventOption = func(*pb.ProcessAnalyticsEventRequest)

func ProcessAnalyticsEventDevKey(devKey string) ProcessAnalyticsEventOption {
	return func(req *pb.ProcessAnalyticsEventRequest) {
		req.DevKey = devKey
	}
}

func (c *Client) ProcessAnalyticsEvent(ctx context.Context, orgKey string, event analyticspb.Event, opts ...ProcessAnalyticsEventOption) (*pb.Reward, error) {
	req := &pb.ProcessAnalyticsEventRequest{
		OrgKey:         orgKey,
		AnalyticsEvent: event,
	}
	for _, opt := range opts {
		opt(req)
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

func ListRewardsStartAt(time time.Time) ListRewardsOption {
	return func(req *pb.ListRewardsRequest) {
		req.StartAt = timestamppb.New(time)
	}
}

func ListRewardsLimit(limit int64) ListRewardsOption {
	return func(req *pb.ListRewardsRequest) {
		req.Limit = limit
	}
}

func (c *Client) ListRewards(ctx context.Context, opts ...ListRewardsOption) ([]*pb.Reward, bool, *time.Time, error) {
	req := &pb.ListRewardsRequest{}
	for _, opt := range opts {
		opt(req)
	}
	res, err := c.fsc.ListRewards(ctx, req)
	if err != nil {
		return nil, false, nil, fmt.Errorf("calling list rewards rpc: %v", err)
	}
	var t *time.Time
	if res.MoreStartAt != nil {
		ts := res.MoreStartAt.AsTime()
		t = &ts
	}
	return res.Rewards, res.More, t, nil
}

func (c *Client) Claim(ctx context.Context, orgKey, claimedBy string, amount int32) (*pb.Claim, error) {
	req := &pb.ClaimRequest{
		OrgKey:    orgKey,
		ClaimedBy: claimedBy,
		Amount:    amount,
	}
	res, err := c.fsc.Claim(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("calling claim rpc: %v", err)
	}
	return res.Claim, nil
}

type FinalizeClaimOption = func(*pb.FinalizeClaimRequest)

func FinalizeClaimTxnCid(txnCid string) FinalizeClaimOption {
	return func(req *pb.FinalizeClaimRequest) {
		req.TxnCid = txnCid
	}
}

func FinalizeClaimFailureMessage(failureMessage string) FinalizeClaimOption {
	return func(req *pb.FinalizeClaimRequest) {
		req.FailureMessage = failureMessage
	}
}

func (c *Client) FinalizeClaim(ctx context.Context, id, orgKey string, opts ...FinalizeClaimOption) error {
	req := &pb.FinalizeClaimRequest{Id: id, OrgKey: orgKey}
	for _, opt := range opts {
		opt(req)
	}
	_, err := c.fsc.FinalizeClaim(ctx, req)
	if err != nil {
		return fmt.Errorf("calling finalize claim rpc: %v", err)
	}
	return nil
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

func ListClaimsStateFilter(state pb.ClaimState) ListClaimsOption {
	return func(req *pb.ListClaimsRequest) {
		req.StateFilter = state
	}
}

func ListClaimsAscending() ListClaimsOption {
	return func(req *pb.ListClaimsRequest) {
		req.Ascending = true
	}
}

func ListClaimsStartAt(time time.Time) ListClaimsOption {
	return func(req *pb.ListClaimsRequest) {
		req.StartAt = timestamppb.New(time)
	}
}

func ListClaimsLimit(limit int64) ListClaimsOption {
	return func(req *pb.ListClaimsRequest) {
		req.Limit = limit
	}
}

func (c *Client) ListClaims(ctx context.Context, opts ...ListClaimsOption) ([]*pb.Claim, bool, *time.Time, error) {
	req := &pb.ListClaimsRequest{}
	for _, opt := range opts {
		opt(req)
	}
	res, err := c.fsc.ListClaims(ctx, req)
	if err != nil {
		return nil, false, nil, fmt.Errorf("calling list claims rpc: %v", err)
	}
	var t *time.Time
	if res.MoreStartAt != nil {
		ts := res.MoreStartAt.AsTime()
		t = &ts
	}
	return res.Claims, res.More, t, nil
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
