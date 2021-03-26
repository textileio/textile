package interfaces

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/ipfs/go-cid"
	pb "github.com/textileio/textile/v2/api/sendfild/pb"
)

type FilecoinClient interface {
	MpoolPushMessage(ctx context.Context, msg *types.Message, spec *api.MessageSendSpec) (*types.SignedMessage, error)
	StateWaitMsg(ctx context.Context, msgc cid.Cid, confidence uint64) (*api.MsgLookup, error)
}

type FilecoinClientBuilder func(context.Context) (FilecoinClient, func(), error)

var ErrTxnNotFound = fmt.Errorf("no txn found")

type TxnStore interface {
	New(ctx context.Context, messageCid, from, to string, amountNanoFil int64) (*pb.Txn, error)
	Get(ctx context.Context, messageCid string) (*pb.Txn, error)
	GetAllPending(ctx context.Context, excludeAlreadyWaiting bool) ([]*pb.Txn, error)
	SetWaiting(ctx context.Context, messageCid string, waiting bool) error
	Fail(ctx context.Context, messageCid, failureMsg string) error
	Activate(ctx context.Context, knownCid, latestCid string) error
	List(ctx context.Context, req *pb.ListTxnsRequest) ([]*pb.Txn, error)
	Summary(ctx context.Context, after, before time.Time) (*pb.SummaryResponse, error)
	io.Closer
}
