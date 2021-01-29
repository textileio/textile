package client

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/textile/v2/api/analyticsd/pb"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/structpb"
)

type config struct {
	active     bool
	email      string
	properties map[string]interface{}
}

type Option = func(*config)

func WithActive() Option {
	return func(c *config) {
		c.active = true
	}
}

func WithEmail(email string) Option {
	return func(c *config) {
		c.email = email
	}
}

func WithProperties(properties map[string]interface{}) Option {
	return func(c *config) {
		c.properties = properties
	}
}

type Client struct {
	asc  pb.AnalyticsServiceClient
	conn *grpc.ClientConn
}

func New(target string, opts ...grpc.DialOption) (*Client, error) {
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating gRPC client conn: %v", err)
	}

	c := pb.NewAnalyticsServiceClient(conn)

	return &Client{
		asc:  c,
		conn: conn,
	}, nil
}

func (c *Client) Identify(ctx context.Context, key string, accountType pb.AccountType, opts ...Option) error {
	if c == nil {
		return nil
	}
	conf := &config{}
	for _, opt := range opts {
		opt(conf)
	}
	req := &pb.IdentifyRequest{
		Key:         key,
		AccountType: accountType,
		Active:      conf.active,
		Email:       conf.email,
	}
	if conf.properties != nil {
		p, err := structpb.NewStruct(conf.properties)
		if err != nil {
			return fmt.Errorf("parsing properties option: %v", err)
		}
		req.Properties = p
	}
	if _, err := c.asc.Identify(ctx, req); err != nil {
		return fmt.Errorf("calling rpc identify: %v", err)
	}
	return nil
}

func (c *Client) Track(ctx context.Context, key string, accountType pb.AccountType, event pb.Event, opts ...Option) error {
	if c == nil {
		return nil
	}
	conf := &config{}
	for _, opt := range opts {
		opt(conf)
	}
	req := &pb.TrackRequest{
		Key:         key,
		AccountType: accountType,
		Event:       event,
		Active:      conf.active,
	}
	if conf.properties != nil {
		p, err := structpb.NewStruct(conf.properties)
		if err != nil {
			return fmt.Errorf("parsing properties option: %v", err)
		}
		req.Properties = p
	}
	if _, err := c.asc.Track(ctx, req); err != nil {
		return fmt.Errorf("calling rpc track: %v", err)
	}
	return nil
}

// FormatUnix converts seconds to string in same format for all analytics requests
func FormatUnix(seconds int64) string {
	return time.Unix(seconds, 0).Format(time.RFC3339)
}

func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	return c.conn.Close()
}
