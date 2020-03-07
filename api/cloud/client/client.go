package client

import (
	"context"

	pb "github.com/textileio/textile/api/cloud/pb"
	"google.golang.org/grpc"
	creds "google.golang.org/grpc/credentials"
)

// Auth supplies authorization credentials.
type Auth struct {
	Token string
	Org   string
}

// Client provides the client api.
type Client struct {
	c    pb.APIClient
	conn *grpc.ClientConn
}

// NewClient starts the client.
func NewClient(target string, creds creds.TransportCredentials) (*Client, error) {
	var opts []grpc.DialOption
	c := credentials{}
	if creds != nil {
		opts = append(opts, grpc.WithTransportCredentials(creds))
		c.secure = true
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	opts = append(opts, grpc.WithPerRPCCredentials(c))
	conn, err := grpc.Dial(target, opts...)
	if err != nil {
		return nil, err
	}
	return &Client{
		c:    pb.NewAPIClient(conn),
		conn: conn,
	}, nil
}

// Close closes the client's grpc connection and cancels any active requests.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Login currently gets or creates a user for the given email address,
// and then waits for email-based verification.
// @todo: Create a dedicated signup flow that collects more info like name, etc.
func (c *Client) Login(ctx context.Context, username, email string) (*pb.LoginReply, error) {
	return c.c.Login(ctx, &pb.LoginRequest{
		Username: username,
		Email:    email,
	})
}

// Logout deletes a remote session.
func (c *Client) Logout(ctx context.Context, auth Auth) error {
	_, err := c.c.Logout(authCtx(ctx, auth), &pb.LogoutRequest{})
	return err
}

// Whoami returns session info.
func (c *Client) Whoami(ctx context.Context, auth Auth) (*pb.WhoamiReply, error) {
	return c.c.Whoami(authCtx(ctx, auth), &pb.WhoamiRequest{})
}

// AddOrg add a new org.
func (c *Client) AddOrg(ctx context.Context, name string, auth Auth) (*pb.GetOrgReply, error) {
	return c.c.AddOrg(authCtx(ctx, auth), &pb.AddOrgRequest{Name: name})
}

// GetOrg returns an org.
func (c *Client) GetOrg(ctx context.Context, auth Auth) (*pb.GetOrgReply, error) {
	return c.c.GetOrg(authCtx(ctx, auth), &pb.GetOrgRequest{})
}

// ListOrgs returns a list of orgs for the current session.
func (c *Client) ListOrgs(ctx context.Context, auth Auth) (*pb.ListOrgsReply, error) {
	return c.c.ListOrgs(authCtx(ctx, auth), &pb.ListOrgsRequest{})
}

// RemoveOrg removes an org.
func (c *Client) RemoveOrg(ctx context.Context, auth Auth) error {
	_, err := c.c.RemoveOrg(authCtx(ctx, auth), &pb.RemoveOrgRequest{})
	return err
}

// InviteToOrg invites the given email to an org.
func (c *Client) InviteToOrg(ctx context.Context, email string, auth Auth) (*pb.InviteToOrgReply, error) {
	return c.c.InviteToOrg(authCtx(ctx, auth), &pb.InviteToOrgRequest{
		Email: email,
	})
}

// LeaveOrg removes the current session dev from an org.
func (c *Client) LeaveOrg(ctx context.Context, auth Auth) error {
	_, err := c.c.LeaveOrg(authCtx(ctx, auth), &pb.LeaveOrgRequest{})
	return err
}

type authKey string

func authCtx(ctx context.Context, auth Auth) context.Context {
	if auth.Token != "" {
		ctx = context.WithValue(ctx, authKey("token"), auth.Token)
	}
	if auth.Org != "" {
		ctx = context.WithValue(ctx, authKey("org"), auth.Org)
	}
	return ctx
}

type credentials struct {
	secure bool
}

func (c credentials) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	md := map[string]string{}
	token, ok := ctx.Value(authKey("token")).(string)
	if ok && token != "" {
		md["authorization"] = "bearer " + token
	}
	org, ok := ctx.Value(authKey("org")).(string)
	if ok && org != "" {
		md["x-org"] = org
	}
	return md, nil
}

func (c credentials) RequireTransportSecurity() bool {
	return c.secure
}
