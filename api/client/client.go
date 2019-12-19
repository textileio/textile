package client

import (
	"context"
	"io"
	"log"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-textile-threads/util"
	pb "github.com/textileio/textile/api/pb"
	"google.golang.org/grpc"
)

// Client provides the client api.
type Client struct {
	client pb.APIClient
	conn   *grpc.ClientConn
}

// NewClient starts the client.
func NewClient(maddr ma.Multiaddr) (*Client, error) {
	addr, err := util.TCPAddrFromMultiAddr(maddr)
	if err != nil {
		return nil, err
	}
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
	}

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		return nil, err
	}
	client := &Client{
		client: pb.NewAPIClient(conn),
		conn:   conn,
	}
	return client, nil
}

// Close closes the client's grpc connection and cancels any active requests.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Login returns an authorization token.
func (c *Client) Login(ctx context.Context, email string) (string, error) {
	stream, err := c.client.Login(ctx, &pb.LoginRequest{Email: email})
	if err != nil {
		return "", err
	}
	token := make(chan string)
	go func() {
		for {
			in, err := stream.Recv()
			if err == io.EOF {
				// read done.
				close(token)
				return
			}
			if err != nil {
				// @todo ensure server errors aren't returned to the CLI
				log.Fatalf("%v", err)
			}
			log.Printf("%v", in.Token)
			token <- in.Token
		}
	}()
	stream.CloseSend()
	// @todo: return message here instead of token
	return <-token, nil
}
