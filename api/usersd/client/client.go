package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/gogo/status"
	"github.com/ipfs/go-cid"
	"github.com/textileio/go-threads/core/thread"
	pb "github.com/textileio/textile/v2/api/usersd/pb"
	"github.com/textileio/textile/v2/threaddb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// Client provides the client api.
type Client struct {
	c    pb.APIServiceClient
	conn *grpc.ClientConn
}

// NewClient starts the client.
func NewClient(target string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		c:    pb.NewAPIServiceClient(conn),
		conn: conn,
	}, nil
}

// Close closes the client's grpc connection and cancels any active requests.
func (c *Client) Close() error {
	return c.conn.Close()
}

// GetThread returns a thread by name.
func (c *Client) GetThread(ctx context.Context, name string) (*pb.GetThreadResponse, error) {
	return c.c.GetThread(ctx, &pb.GetThreadRequest{
		Name: name,
	})
}

// ListThreads returns a list of threads.
// Threads can be created using the threads or threads network client.
func (c *Client) ListThreads(ctx context.Context) (*pb.ListThreadsResponse, error) {
	return c.c.ListThreads(ctx, &pb.ListThreadsRequest{})
}

// SetupMailbox creates inbox and sentbox threads needed user mail.
func (c *Client) SetupMailbox(ctx context.Context) (mailbox thread.ID, err error) {
	res, err := c.c.SetupMailbox(ctx, &pb.SetupMailboxRequest{})
	if err != nil {
		return
	}
	return thread.Cast(res.MailboxId)
}

// Message is the client side representation of a mailbox message.
// Signature corresponds to the encrypted body.
// Use message.Open to get the plaintext body.
type Message struct {
	ID        string        `json:"_id"`
	From      thread.PubKey `json:"from"`
	To        thread.PubKey `json:"to"`
	Body      []byte        `json:"body"`
	Signature []byte        `json:"signature"`
	CreatedAt time.Time     `json:"created_at"`
	ReadAt    time.Time     `json:"read_at,omitempty"`
}

// Open decrypts the message body with identity.
func (m Message) Open(ctx context.Context, id thread.Identity) ([]byte, error) {
	return id.Decrypt(ctx, m.Body)
}

// IsRead returns whether or not the message has been read.
func (m Message) IsRead() bool {
	return !m.ReadAt.IsZero()
}

// UnmarshalInstance unmarshals the message from its ThreadDB instance data.
// This will return an error if the message signature fails verification.
func (m Message) UnmarshalInstance(data []byte) error {
	// InboxMessage works for both inbox and sentbox messages (it contains a superset of SentboxMessage fields)
	var tm threaddb.InboxMessage
	if err := json.Unmarshal(data, &tm); err != nil {
		return err
	}
	body, err := base64.StdEncoding.DecodeString(tm.Body)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(tm.Signature)
	if err != nil {
		return err
	}
	from := &thread.Libp2pPubKey{}
	if err := from.UnmarshalString(tm.From); err != nil {
		return fmt.Errorf("from public key is invalid")
	}
	ok, err := from.Verify(body, sig)
	if !ok || err != nil {
		return fmt.Errorf("bad message signature")
	}
	to := &thread.Libp2pPubKey{}
	if err := to.UnmarshalString(tm.To); err != nil {
		return fmt.Errorf("to public key is invalid")
	}
	readAt := time.Time{}
	if tm.ReadAt > 0 {
		readAt = time.Unix(0, tm.ReadAt)
	}
	m.ID = tm.ID
	m.From = from
	m.To = to
	m.Body = body
	m.Signature = sig
	m.CreatedAt = time.Unix(0, tm.CreatedAt)
	m.ReadAt = readAt
	return nil
}

// SendMessage sends the message body to a recipient.
func (c *Client) SendMessage(ctx context.Context, from thread.Identity, to thread.PubKey, body []byte) (msg Message, err error) {
	fromBody, err := from.GetPublic().Encrypt(body)
	if err != nil {
		return msg, err
	}
	fromSig, err := from.Sign(ctx, fromBody)
	if err != nil {
		return msg, err
	}
	toBody, err := to.Encrypt(body)
	if err != nil {
		return msg, err
	}
	toSig, err := from.Sign(ctx, toBody)
	if err != nil {
		return msg, err
	}
	res, err := c.c.SendMessage(ctx, &pb.SendMessageRequest{
		To:            to.String(),
		ToBody:        toBody,
		ToSignature:   toSig,
		FromBody:      fromBody,
		FromSignature: fromSig,
	})
	if err != nil {
		return msg, err
	}
	return Message{
		ID:        res.Id,
		From:      from.GetPublic(),
		To:        to,
		Body:      fromBody,
		Signature: fromSig,
		CreatedAt: time.Unix(0, res.CreatedAt),
	}, nil
}

// ListInboxMessages lists messages from the inbox.
// Use options to paginate with seek and limit and filter by read status.
func (c *Client) ListInboxMessages(ctx context.Context, opts ...ListOption) ([]Message, error) {
	args := &listOptions{
		status: All,
	}
	for _, opt := range opts {
		opt(args)
	}
	var s pb.ListInboxMessagesRequest_Status
	switch args.status {
	case All:
		s = pb.ListInboxMessagesRequest_STATUS_ALL
	case Read:
		s = pb.ListInboxMessagesRequest_STATUS_READ
	case Unread:
		s = pb.ListInboxMessagesRequest_STATUS_UNREAD
	default:
		return nil, fmt.Errorf("unknown status: %v", args.status)
	}
	res, err := c.c.ListInboxMessages(ctx, &pb.ListInboxMessagesRequest{
		Seek:      args.seek,
		Limit:     int64(args.limit),
		Ascending: args.ascending,
		Status:    s,
	})
	if err != nil {
		return nil, err
	}
	return handleMessageList(res.Messages)
}

// ListSentboxMessages lists messages from the sentbox.
// Use options to paginate with seek and limit.
func (c *Client) ListSentboxMessages(ctx context.Context, opts ...ListOption) ([]Message, error) {
	args := &listOptions{
		status: All,
	}
	for _, opt := range opts {
		opt(args)
	}
	res, err := c.c.ListSentboxMessages(ctx, &pb.ListSentboxMessagesRequest{
		Seek:  args.seek,
		Limit: int64(args.limit),
	})
	if err != nil {
		return nil, err
	}
	return handleMessageList(res.Messages)
}

func handleMessageList(list []*pb.Message) ([]Message, error) {
	msgs := make([]Message, len(list))
	var err error
	for i, m := range list {
		msgs[i], err = messageFromPb(m)
		if err != nil {
			return nil, err
		}
	}
	return msgs, nil
}

func messageFromPb(m *pb.Message) (msg Message, err error) {
	from := &thread.Libp2pPubKey{}
	if err := from.UnmarshalString(m.From); err != nil {
		return msg, fmt.Errorf("from public key is invalid")
	}
	ok, err := from.Verify(m.Body, m.Signature)
	if !ok || err != nil {
		return msg, fmt.Errorf("bad message signature")
	}
	to := &thread.Libp2pPubKey{}
	if err := to.UnmarshalString(m.To); err != nil {
		return msg, fmt.Errorf("to public key is invalid")
	}
	readAt := time.Time{}
	if m.ReadAt > 0 {
		readAt = time.Unix(0, m.ReadAt)
	}
	return Message{
		ID:        m.Id,
		From:      from,
		To:        to,
		Body:      m.Body,
		Signature: m.Signature,
		CreatedAt: time.Unix(0, m.CreatedAt),
		ReadAt:    readAt,
	}, nil
}

// ReadInboxMessage marks a message as read by ID.
func (c *Client) ReadInboxMessage(ctx context.Context, id string) error {
	_, err := c.c.ReadInboxMessage(ctx, &pb.ReadInboxMessageRequest{
		Id: id,
	})
	return err
}

// DeleteInboxMessage deletes an inbox message by ID.
func (c *Client) DeleteInboxMessage(ctx context.Context, id string) error {
	_, err := c.c.DeleteInboxMessage(ctx, &pb.DeleteInboxMessageRequest{
		Id: id,
	})
	return err
}

// DeleteSentboxMessage deletes a sent message by ID.
func (c *Client) DeleteSentboxMessage(ctx context.Context, id string) error {
	_, err := c.c.DeleteSentboxMessage(ctx, &pb.DeleteSentboxMessageRequest{
		Id: id,
	})
	return err
}

// GetUsage returns current billing and usage information.
func (c *Client) GetUsage(ctx context.Context, opts ...UsageOption) (*pb.GetUsageResponse, error) {
	args := &usageOptions{}
	for _, opt := range opts {
		opt(args)
	}
	return c.c.GetUsage(ctx, &pb.GetUsageRequest{
		Key: args.key,
	})
}

// ArchivesLs list all imported archives.
func (c *Client) ArchivesLs(ctx context.Context) (*pb.ArchivesLsResponse, error) {
	req := &pb.ArchivesLsRequest{}
	return c.c.ArchivesLs(ctx, req)
}

// ArchivesImport imports deals information for a Cid.
func (c *Client) ArchivesImport(ctx context.Context, dataCid cid.Cid, dealIDs []uint64) error {
	req := &pb.ArchivesImportRequest{
		Cid:     dataCid.String(),
		DealIds: dealIDs,
	}
	_, err := c.c.ArchivesImport(ctx, req)
	return err
}

// ArchiveRetrievalLs lists existing retrievals.
func (c *Client) ArchiveRetrievalLs(ctx context.Context) (*pb.ArchiveRetrievalLsResponse, error) {
	req := &pb.ArchiveRetrievalLsRequest{}
	return c.c.ArchiveRetrievalLs(ctx, req)
}

// ArchiveRetrievalLogs returns the existing logs from the retrieval.
func (c *Client) ArchiveRetrievalLogs(ctx context.Context, id string, ch chan<- string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stream, err := c.c.ArchiveRetrievalLogs(ctx, &pb.ArchiveRetrievalLogsRequest{Id: id})
	if err != nil {
		return err
	}
	for {
		reply, err := stream.Recv()
		if err == io.EOF || status.Code(err) == codes.Canceled {
			break
		}
		if err != nil {
			return err
		}
		ch <- reply.Msg
	}
	return nil
}
