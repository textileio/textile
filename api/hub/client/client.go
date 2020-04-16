package client

import (
	"context"

	pb "github.com/textileio/textile/api/hub/pb"
	"google.golang.org/grpc"
)

// Client provides the client api.
type Client struct {
	c    pb.APIClient
	conn *grpc.ClientConn
}

// NewClient starts the client.
func NewClient(target string, opts ...grpc.DialOption) (*Client, error) {
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
func (c *Client) Login(ctx context.Context, username, email string) (*pb.LoginReply, error) {
	return c.c.Login(ctx, &pb.LoginRequest{
		Username: username,
		Email:    email,
	})
}

// Logout deletes a remote session.
func (c *Client) Logout(ctx context.Context) error {
	_, err := c.c.Logout(ctx, &pb.LogoutRequest{})
	return err
}

// Whoami returns session info.
func (c *Client) Whoami(ctx context.Context) (*pb.WhoamiReply, error) {
	return c.c.Whoami(ctx, &pb.WhoamiRequest{})
}

// GetThread returns a thread by name.
func (c *Client) GetThread(ctx context.Context, name string) (*pb.GetThreadReply, error) {
	return c.c.GetThread(ctx, &pb.GetThreadRequest{
		Name: name,
	})
}

// ListThreads returns a list of threads.
// Threads can be created using the threads or threads network client.
func (c *Client) ListThreads(ctx context.Context) (*pb.ListThreadsReply, error) {
	return c.c.ListThreads(ctx, &pb.ListThreadsRequest{})
}

// CreateKey creates a new key for the current session.
func (c *Client) CreateKey(ctx context.Context) (*pb.GetKeyReply, error) {
	return c.c.CreateKey(ctx, &pb.CreateKeyRequest{})
}

// InvalidateKey marks a key as invalid.
// New threads cannot be created with an invalid key.
func (c *Client) InvalidateKey(ctx context.Context, key string) error {
	_, err := c.c.InvalidateKey(ctx, &pb.InvalidateKeyRequest{Key: key})
	return err
}

// ListKeys returns a list of keys for the current session.
func (c *Client) ListKeys(ctx context.Context) (*pb.ListKeysReply, error) {
	return c.c.ListKeys(ctx, &pb.ListKeysRequest{})
}

// CreateOrg creates a new org by name.
func (c *Client) CreateOrg(ctx context.Context, name string) (*pb.GetOrgReply, error) {
	return c.c.CreateOrg(ctx, &pb.CreateOrgRequest{Name: name})
}

// GetOrg returns an org.
func (c *Client) GetOrg(ctx context.Context) (*pb.GetOrgReply, error) {
	return c.c.GetOrg(ctx, &pb.GetOrgRequest{})
}

// ListOrgs returns a list of orgs for the current session.
func (c *Client) ListOrgs(ctx context.Context) (*pb.ListOrgsReply, error) {
	return c.c.ListOrgs(ctx, &pb.ListOrgsRequest{})
}

// RemoveOrg removes an org.
func (c *Client) RemoveOrg(ctx context.Context) error {
	_, err := c.c.RemoveOrg(ctx, &pb.RemoveOrgRequest{})
	return err
}

// InviteToOrg invites the given email to an org.
func (c *Client) InviteToOrg(ctx context.Context, email string) (*pb.InviteToOrgReply, error) {
	return c.c.InviteToOrg(ctx, &pb.InviteToOrgRequest{
		Email: email,
	})
}

// LeaveOrg removes the current session dev from an org.
func (c *Client) LeaveOrg(ctx context.Context) error {
	_, err := c.c.LeaveOrg(ctx, &pb.LeaveOrgRequest{})
	return err
}
