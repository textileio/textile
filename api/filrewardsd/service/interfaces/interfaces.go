package interfaces

import (
	"context"
	"time"

	"github.com/textileio/go-threads/core/thread"
	powpb "github.com/textileio/powergate/v2/api/gen/powergate/user/v1"
	analytics "github.com/textileio/textile/v2/api/analyticsd/client"
	analyticspb "github.com/textileio/textile/v2/api/analyticsd/pb"
	pb "github.com/textileio/textile/v2/api/filrewardsd/pb"
	sendfil "github.com/textileio/textile/v2/api/sendfild/client"
	sendfilpb "github.com/textileio/textile/v2/api/sendfild/pb"
	mdb "github.com/textileio/textile/v2/mongodb"
)

type RewardStore interface {
	New(ctx context.Context, orgKey, devKey string, t pb.RewardType, factor, baseNanoFilReward int64) (*pb.Reward, error)
	All(ctx context.Context) ([]*pb.Reward, error)
	List(ctx context.Context, req *pb.ListRewardsRequest) ([]*pb.Reward, error)
	TotalNanoFilRewarded(ctx context.Context, orgKey string) (int64, error)
}

type Claim struct {
	ID            string
	OrgKey        string
	ClaimedBy     string
	AmountNanoFil int64
	TxnCid        string
	CreatedAt     time.Time
}

type ListConfig struct {
	OrgKeyFilter    string
	ClaimedByFilter string
	Ascending       bool
	PageSize        int64
	Page            int64
}

type ClaimStore interface {
	New(ctx context.Context, orgKey, claimedBy string, amountNanoFil int64, msgCid string) (*Claim, error)
	List(ctx context.Context, conf ListConfig) ([]*Claim, error)
}

type AccountStore interface {
	Get(ctx context.Context, key thread.PubKey) (*mdb.Account, error)
}

type Analytics interface {
	Track(ctx context.Context, key string, accountType analyticspb.AccountType, event analyticspb.Event, opts ...analytics.Option) error
}

type SendFil interface {
	SendFil(ctx context.Context, from, to string, amountNanoFil int64, opts ...sendfil.SendFilOption) (*sendfilpb.Txn, error)
	ListTxns(ctx context.Context, opts ...sendfil.ListTxnsOption) ([]*sendfilpb.Txn, error)
}

type Powergate interface {
	Addresses(ctx context.Context) (*powpb.AddressesResponse, error)
}
