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
	conn *grpc.ClientConn
}

// NewClient starts the client.
func NewClient(maddr ma.Multiaddr) (*Client, error) {
	addr, err := util.TCPAddrFromMultiAddr(maddr)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithPerRPCCredentials(tokenAuth{}))
	if err != nil {
		return nil, err
	}
	return &Client{conn: conn}, nil
}

// Close closes the client's grpc connection and cancels any active requests.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Login currently gets or creates a user for the given email address,
// and then waits for email-based verification.
// @todo: Create a dedicated signup flow that collects more info like name, etc.
func (c *Client) Login(ctx context.Context, email string) (*pb.LoginReply, error) {
	return pb.NewAPIClient(c.conn).Login(ctx, &pb.LoginRequest{Email: email})
}

// AddTeam add a new team.
func (c *Client) AddTeam(ctx context.Context, name, token string) (*pb.AddTeamReply, error) {
	ctx = context.WithValue(ctx, authKey("token"), token)
	return pb.NewAPIClient(c.conn).AddTeam(ctx, &pb.AddTeamRequest{Name: name})
}

// AddProject add a new project under the given scope.
func (c *Client) AddProject(ctx context.Context, name, token, scope string) (*pb.AddProjectReply, error) {
	ctx = context.WithValue(ctx, authKey("token"), token)
	ctx = context.WithValue(ctx, authKey("scope"), scope)
	return pb.NewAPIClient(c.conn).AddProject(ctx, &pb.AddProjectRequest{
		Name: name,
	})
}

type tokenAuth struct{}

type authKey string

func (t tokenAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	md := map[string]string{}
	token, ok := ctx.Value(authKey("token")).(string)
	if ok && token != "" {
		md["Authorization"] = "Bearer " + token
	}
	scope, ok := ctx.Value(authKey("scope")).(string)
	if ok && scope != "" {
		md["X-Scope"] = scope
	}
	return md, nil
}

func (tokenAuth) RequireTransportSecurity() bool {
	return false
}
