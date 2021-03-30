package client

import (
	"context"
	"time"

	"github.com/textileio/textile/v2/api/sendfild/pb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Client is the sendfil client.
type Client struct {
	c    pb.SendFilServiceClient
	conn *grpc.ClientConn
}

// New creates a new sendfil client.
func New(conn *grpc.ClientConn) (*Client, error) {
	c := pb.NewSendFilServiceClient(conn)

	return &Client{
		c:    c,
		conn: conn,
	}, nil
}

// SendFilOption controls the behavior of calling SendFil.
type SendFilOption = func(*pb.SendFilRequest)

// SendFilWait specifies to block the return of SendFil until the underlying Txn message is active on chain or monitoring times out.
func SendFilWait() SendFilOption {
	return func(req *pb.SendFilRequest) {
		req.Wait = true
	}
}

// SendFil sends FIL from one addres to another with the provided options.
func (c *Client) SendFil(ctx context.Context, from, to string, amountNanoFil int64, opts ...SendFilOption) (*pb.Txn, error) {
	req := &pb.SendFilRequest{
		From:          from,
		To:            to,
		AmountNanoFil: amountNanoFil,
	}
	for _, opt := range opts {
		opt(req)
	}
	res, err := c.c.SendFil(ctx, req)
	if err != nil {
		return nil, err
	}
	return res.Txn, nil
}

// GetTxnOption contorls the behavior of calling GetTxn.
type GetTxnOption = func(*pb.GetTxnRequest)

// GetTxnWait specifies to block the return of GetTxn until the underlying Txn message is active on chain or monitoring times out.
func GetTxnWait() GetTxnOption {
	return func(req *pb.GetTxnRequest) {
		req.Wait = true
	}
}

// GetTxn gets a Txn using the provided options.
func (c *Client) GetTxn(ctx context.Context, messageCid string, opts ...GetTxnOption) (*pb.Txn, error) {
	req := &pb.GetTxnRequest{
		MessageCid: messageCid,
	}
	for _, opt := range opts {
		opt(req)
	}
	res, err := c.c.GetTxn(ctx, req)
	if err != nil {
		return nil, err
	}
	return res.Txn, nil
}

// ListTxnsOption controls the behavior of calling ListTxns.
type ListTxnsOption = func(*pb.ListTxnsRequest)

// ListTxnsMessageCids filters results to Txns for the specified message cids.
func ListTxnsMessageCids(cids []string) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.MessageCidsFilter = cids
	}
}

// ListTxnsFrom filters results to Txns from the specified address.
func ListTxnsFrom(from string) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.FromFilter = from
	}
}

// ListTxnsTo filters results to Txns to the specified address.
func ListTxnsTo(to string) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.ToFilter = to
	}
}

// ListTxnsInvolvingAddress filters results to Txns from or to the specified address.
func ListTxnsInvolvingAddress(involving string) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.InvolvingAddressFilter = involving
	}
}

// ListTxnsAmountNanoFilLt filters results to Txns less than the specified amount.
func ListTxnsAmountNanoFilLt(amountNanoFil int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.AmountNanoFilLtFilter = amountNanoFil
	}
}

// ListTxnsAmountNanoFilGt filters results to Txns greater than the specified amount.
func ListTxnsAmountNanoFilGt(amountNanoFil int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.AmountNanoFilGtFilter = amountNanoFil
	}
}

// ListTxnsAmountNanoFilLteq filters results to Txns less than or equal to the specified amount.
func ListTxnsAmountNanoFilLteq(amountNanoFil int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.AmountNanoFilLteqFilter = amountNanoFil
	}
}

// ListTxnsAmountNanoFilGteq filters results to Txns greater than or equal to the specified amount.
func ListTxnsAmountNanoFilGteq(amountNanoFil int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.AmountNanoFilGteqFilter = amountNanoFil
	}
}

// ListTxnsAmountNanoFilEq filters results to Txns equal to the specified amount.
func ListTxnsAmountNanoFilEq(amountNanoFil int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.AmountNanoFilEqFilter = amountNanoFil
	}
}

// ListTxnsMessageState filters results to Txns with specified message state.
func ListTxnsMessageState(messageState pb.MessageState) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.MessageStateFilter = messageState
	}
}

// ListTxnsWaiting filters results to Txns with specified waiting state.
func ListTxnsWaiting(waitingFilter pb.WaitingFilter) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.WaitingFilter = waitingFilter
	}
}

// ListTxnsCreatedAfter filters results to Txns created after the specified time.
func ListTxnsCreatedAfter(time time.Time) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.CreatedAfter = timestamppb.New(time)
	}
}

// ListTxnsCreatedBefore filters results to Txns created before the specified time.
func ListTxnsCreatedBefore(time time.Time) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.CreatedBefore = timestamppb.New(time)
	}
}

// ListTxnsUpdatedAfter filters results to Txns updated after the specified time.
func ListTxnsUpdatedAfter(time time.Time) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.UpdatedAfter = timestamppb.New(time)
	}
}

// ListTxnsUpdatedBefore filters results to Txns updated before the specified time.
func ListTxnsUpdatedBefore(time time.Time) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.UpdatedBefore = timestamppb.New(time)
	}
}

// ListTxnsAscending causes results to be sorted by created time ascending, default is descending.
func ListTxnsAscending() ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.Ascending = true
	}
}

// ListTxnsPageSize controls the number of results returned in a page, or single call.
func ListTxnsPageSize(pageSize int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.PageSize = pageSize
	}
}

// ListTxnsPage specifies the 0 based page of results to fetch.
func ListTxnsPage(page int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.Page = page
	}
}

// ListTxns lists Txns using the provided options.
func (c *Client) ListTxns(ctx context.Context, opts ...ListTxnsOption) ([]*pb.Txn, error) {
	req := &pb.ListTxnsRequest{}
	for _, opt := range opts {
		opt(req)
	}
	res, err := c.c.ListTxns(ctx, req)
	if err != nil {
		return nil, err
	}
	return res.Txns, nil
}

// Close closes the Client.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.conn.Close()
}
