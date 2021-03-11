package client

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/textile/v2/api/sendfild/pb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Client struct {
	c    pb.SendFilServiceClient
	conn *grpc.ClientConn
}

func New(target string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating gRPC client conn: %v", err)
	}

	c := pb.NewSendFilServiceClient(conn)

	return &Client{
		c:    c,
		conn: conn,
	}, nil
}

type SendFilOption = func(*pb.SendFilRequest)

func SendFilWait() SendFilOption {
	return func(req *pb.SendFilRequest) {
		req.Wait = true
	}
}

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

type GetTxnOption = func(*pb.GetTxnRequest)

func GetTxnWait() GetTxnOption {
	return func(req *pb.GetTxnRequest) {
		req.Wait = true
	}
}

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

type ListTxnsOption = func(*pb.ListTxnsRequest)

func ListTxnsFrom(from string) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.FromFilter = from
	}
}

func ListTxnsTo(to string) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.ToFilter = to
	}
}

func ListTxnsInvolvingAddress(involving string) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.InvolvingAddressFilter = involving
	}
}

func ListTxnsAmountNanoFilLt(amountNanoFil int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.AmountNanoFilLtFilter = amountNanoFil
	}
}

func ListTxnsAmountNanoFilGt(amountNanoFil int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.AmountNanoFilGtFilter = amountNanoFil
	}
}

func ListTxnsAmountNanoFilLteq(amountNanoFil int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.AmountNanoFilLteqFilter = amountNanoFil
	}
}

func ListTxnsAmountNanoFilGteq(amountNanoFil int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.AmountNanoFilGteqFilter = amountNanoFil
	}
}

func ListTxnsAmountNanoFilEq(amountNanoFil int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.AmountNanoFilEqFilter = amountNanoFil
	}
}

func ListTxnsMessageState(messageState pb.MessageState) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.MessageStateFilter = messageState
	}
}

func ListTxnsWaiting(waitingFilter pb.WaitingFilter) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.WaitingFilter = waitingFilter
	}
}

func ListTxnsCreatedAfter(time time.Time) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.CreatedAfter = timestamppb.New(time)
	}
}

func ListTxnsCreatedBefore(time time.Time) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.CreatedBefore = timestamppb.New(time)
	}
}

func ListTxnsUpdatedAfter(time time.Time) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.UpdatedAfter = timestamppb.New(time)
	}
}

func ListTxnsUpdatedBefore(time time.Time) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.UpdatedBefore = timestamppb.New(time)
	}
}

func ListTxnsAscending() ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.Ascending = true
	}
}

func ListTxnsPageSize(pageSize int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.PageSize = pageSize
	}
}

func ListTxnsPage(page int64) ListTxnsOption {
	return func(req *pb.ListTxnsRequest) {
		req.Page = page
	}
}

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

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.conn.Close()
}
