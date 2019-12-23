package client

import (
	"context"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/util"
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

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
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
func (c *Client) Login(ctx context.Context, email string) (*pb.LoginReply, error) {
	stream, err := c.client.Login(ctx, &pb.LoginRequest{Email: email})
	if err != nil {
		return nil, err
	}
	return stream.Recv()
}

// AddTeam add a new team under the current scope.
func (c *Client) AddTeam(ctx context.Context, name string) (*pb.AddTeamReply, error) {
	resp, err := c.client.AddTeam(ctx, &pb.AddTeamRequest{Name: name})
	return resp, err
}

// AddProject add a new project under the current scope.
func (c *Client) AddProject(ctx context.Context, name string, scopeID string) (*pb.AddProjectReply, error) {
	resp, err := c.client.AddProject(ctx, &pb.AddProjectRequest{
		Name:    name,
		ScopeID: scopeID,
	})
	return resp, err
}
