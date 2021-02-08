package client

import (
	"time"

	filrewardspb "github.com/textileio/textile/v2/api/filrewardsd/pb"
	pb "github.com/textileio/textile/v2/api/usersd/pb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type listOptions struct {
	seek      string
	limit     int
	ascending bool
	status    Status
}

type ListOption func(*listOptions)

// WithSeek starts listing from the given ID.
func WithSeek(id string) ListOption {
	return func(args *listOptions) {
		args.seek = id
	}
}

// WithLimit limits the number of list messages results.
func WithLimit(limit int) ListOption {
	return func(args *listOptions) {
		args.limit = limit
	}
}

// WithAscending lists messages by ascending order.
func WithAscending(asc bool) ListOption {
	return func(args *listOptions) {
		args.ascending = asc
	}
}

// Status indicates message read status.
type Status int

const (
	// All includes read and unread messages.
	All Status = iota
	// Read is only read messages.
	Read
	// Unread is only unread messages.
	Unread
)

// WithStatus filters messages by read status.
// Note: Only applies to inbox messages.
func WithStatus(s Status) ListOption {
	return func(args *listOptions) {
		args.status = s
	}
}

type usageOptions struct {
	key string
}

type UsageOption func(*usageOptions)

// WithPubKey returns usage info for the public key.
func WithPubKey(key string) UsageOption {
	return func(args *usageOptions) {
		args.key = key
	}
}

type ListFilRewardsOption = func(*pb.ListFilRewardsRequest)

func ListFilRewardsUnlockedByDev() ListFilRewardsOption {
	return func(req *pb.ListFilRewardsRequest) {
		req.UnlockedByDev = true
	}
}

func ListFilRewardsRewardTypeFilter(rewardType filrewardspb.RewardType) ListFilRewardsOption {
	return func(req *pb.ListFilRewardsRequest) {
		req.RewardTypeFilter = rewardType
	}
}

func ListFilRewardsAscending() ListFilRewardsOption {
	return func(req *pb.ListFilRewardsRequest) {
		req.Ascending = true
	}
}

func ListFilRewardsStartAt(time time.Time) ListFilRewardsOption {
	return func(req *pb.ListFilRewardsRequest) {
		req.StartAt = timestamppb.New(time)
	}
}

func ListFilRewardsLimit(limit int64) ListFilRewardsOption {
	return func(req *pb.ListFilRewardsRequest) {
		req.Limit = limit
	}
}

type ListClaimsOption = func(*pb.ListFilClaimsRequest)

func ListFilClaimsClaimedByDev() ListClaimsOption {
	return func(req *pb.ListFilClaimsRequest) {
		req.ClaimedByDev = true
	}
}

func ListFilClaimsStateFilter(state filrewardspb.ClaimState) ListClaimsOption {
	return func(req *pb.ListFilClaimsRequest) {
		req.StateFilter = state
	}
}

func ListFilClaimsAscending() ListClaimsOption {
	return func(req *pb.ListFilClaimsRequest) {
		req.Ascending = true
	}
}

func ListFilClaimsStartAt(time time.Time) ListClaimsOption {
	return func(req *pb.ListFilClaimsRequest) {
		req.StartAt = timestamppb.New(time)
	}
}

func ListFilClaimsLimit(limit int64) ListClaimsOption {
	return func(req *pb.ListFilClaimsRequest) {
		req.Limit = limit
	}
}
