package client

import (
	filrewardspb "github.com/textileio/textile/v2/api/filrewardsd/pb"
	pb "github.com/textileio/textile/v2/api/hubd/pb"
)

type listOptions struct {
	offset int64
	limit  int64
}

type ListOption func(*listOptions)

// WithOffset is used to fetch the next page when paginating.
func WithOffset(offset int64) ListOption {
	return func(args *listOptions) {
		args.offset = offset
	}
}

// WithLimit is used to set a page size when paginating.
func WithLimit(limit int64) ListOption {
	return func(args *listOptions) {
		args.limit = limit
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

func ListFilRewardsPage(page int64) ListFilRewardsOption {
	return func(req *pb.ListFilRewardsRequest) {
		req.Page = page
	}
}

func ListFilRewardsPageSize(pageSize int64) ListFilRewardsOption {
	return func(req *pb.ListFilRewardsRequest) {
		req.PageSize = pageSize
	}
}

type ListFilClaimsOption = func(*pb.ListFilClaimsRequest)

func ListFilClaimsClaimedByDev() ListFilClaimsOption {
	return func(req *pb.ListFilClaimsRequest) {
		req.ClaimedByDev = true
	}
}

func ListFilClaimsAscending() ListFilClaimsOption {
	return func(req *pb.ListFilClaimsRequest) {
		req.Ascending = true
	}
}

func ListFilClaimsPage(page int64) ListFilClaimsOption {
	return func(req *pb.ListFilClaimsRequest) {
		req.Page = page
	}
}

func ListFilClaimsPageSize(pageSize int64) ListFilClaimsOption {
	return func(req *pb.ListFilClaimsRequest) {
		req.PageSize = pageSize
	}
}
