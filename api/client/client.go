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
	addr string
}

// NewClient starts the client.
func NewClient(maddr ma.Multiaddr) (*Client, error) {
	addr, err := util.TCPAddrFromMultiAddr(maddr)
	if err != nil {
		return nil, err
	}
	return &Client{addr: addr}, nil
}

// Login returns an authorization token.
func (c *Client) Login(ctx context.Context, email string) (*pb.LoginReply, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	stream, err := pb.NewAPIClient(conn).Login(ctx, &pb.LoginRequest{Email: email})
	if err != nil {
		return nil, err
	}
	return stream.Recv()
}

// AddTeam add a new team under the current scope.
func (c *Client) AddTeam(ctx context.Context, name, token string) (*pb.AddTeamReply, error) {
	conn, err := c.dialWithToken(token, "")
	if err != nil {
		return nil, err
	}
	resp, err := pb.NewAPIClient(conn).AddTeam(ctx, &pb.AddTeamRequest{Name: name})
	return resp, err
}

// AddProject add a new project under the current scope.
func (c *Client) AddProject(ctx context.Context, name, token, scope string) (*pb.AddProjectReply, error) {
	conn, err := c.dialWithToken(token, scope)
	if err != nil {
		return nil, err
	}
	resp, err := pb.NewAPIClient(conn).AddProject(ctx, &pb.AddProjectRequest{
		Name: name,
	})
	return resp, err
}

type tokenAuth struct {
	token string
	scope string
}

func (t tokenAuth) GetRequestMetadata(ctx context.Context, in ...string) (map[string]string, error) {
	return map[string]string{
		"Authorization": "Bearer " + t.token,
		"X-Scope":       t.scope,
	}, nil
}

func (tokenAuth) RequireTransportSecurity() bool {
	return false
}

func (c *Client) dial() (*grpc.ClientConn, error) {
	return grpc.Dial(c.addr, grpc.WithInsecure())
}

func (c *Client) dialWithToken(token, scope string) (*grpc.ClientConn, error) {
	return grpc.Dial(c.addr, grpc.WithInsecure(), grpc.WithPerRPCCredentials(tokenAuth{
		token: token,
		scope: scope,
	}))
}
