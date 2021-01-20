package client

import (
	"context"

	pb "github.com/textileio/textile/v2/api/hubd/pb"
	"google.golang.org/grpc"
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

// Info provides API build information.
func (c *Client) BuildInfo(ctx context.Context) (*pb.BuildInfoResponse, error) {
	return c.c.BuildInfo(ctx, &pb.BuildInfoRequest{})
}

// Signup creates a new user and returns a session.
// This method will block and wait for email-based verification.
func (c *Client) Signup(ctx context.Context, username, email string) (*pb.SignupResponse, error) {
	return c.c.Signup(ctx, &pb.SignupRequest{
		Username: username,
		Email:    email,
	})
}

// Signin returns a session for an existing username or email.
// This method will block and wait for email-based verification.
func (c *Client) Signin(ctx context.Context, usernameOrEmail string) (*pb.SigninResponse, error) {
	return c.c.Signin(ctx, &pb.SigninRequest{
		UsernameOrEmail: usernameOrEmail,
	})
}

// Signout deletes a session.
func (c *Client) Signout(ctx context.Context) error {
	_, err := c.c.Signout(ctx, &pb.SignoutRequest{})
	return err
}

// GetSessionInfo returns session info.
func (c *Client) GetSessionInfo(ctx context.Context) (*pb.GetSessionInfoResponse, error) {
	return c.c.GetSessionInfo(ctx, &pb.GetSessionInfoRequest{})
}

// GetIdentity returns the identity of the current session.
func (c *Client) GetIdentity(ctx context.Context) (*pb.GetIdentityResponse, error) {
	return c.c.GetIdentity(ctx, &pb.GetIdentityRequest{})
}

// CreateKey creates a new key for the current session.
func (c *Client) CreateKey(ctx context.Context, keyType pb.KeyType, secure bool) (*pb.CreateKeyResponse, error) {
	return c.c.CreateKey(ctx, &pb.CreateKeyRequest{
		Type:   keyType,
		Secure: secure,
	})
}

// InvalidateKey marks a key as invalid.
// New threads cannot be created with an invalid key.
func (c *Client) InvalidateKey(ctx context.Context, key string) error {
	_, err := c.c.InvalidateKey(ctx, &pb.InvalidateKeyRequest{Key: key})
	return err
}

// ListKeys returns a list of keys for the current session.
func (c *Client) ListKeys(ctx context.Context) (*pb.ListKeysResponse, error) {
	return c.c.ListKeys(ctx, &pb.ListKeysRequest{})
}

// CreateOrg creates a new org by name.
func (c *Client) CreateOrg(ctx context.Context, name string) (*pb.CreateOrgResponse, error) {
	return c.c.CreateOrg(ctx, &pb.CreateOrgRequest{Name: name})
}

// GetOrg returns an org.
func (c *Client) GetOrg(ctx context.Context) (*pb.GetOrgResponse, error) {
	return c.c.GetOrg(ctx, &pb.GetOrgRequest{})
}

// ListOrgs returns a list of orgs for the current session.
func (c *Client) ListOrgs(ctx context.Context) (*pb.ListOrgsResponse, error) {
	return c.c.ListOrgs(ctx, &pb.ListOrgsRequest{})
}

// RemoveOrg removes an org.
func (c *Client) RemoveOrg(ctx context.Context) error {
	_, err := c.c.RemoveOrg(ctx, &pb.RemoveOrgRequest{})
	return err
}

// InviteToOrg invites the given email to an org.
func (c *Client) InviteToOrg(ctx context.Context, email string) (*pb.InviteToOrgResponse, error) {
	return c.c.InviteToOrg(ctx, &pb.InviteToOrgRequest{
		Email: email,
	})
}

// LeaveOrg removes the current session dev from an org.
func (c *Client) LeaveOrg(ctx context.Context) error {
	_, err := c.c.LeaveOrg(ctx, &pb.LeaveOrgRequest{})
	return err
}

// SetupBilling (re-)enables billing for an account, enabling
// usage beyond the free quotas.
func (c *Client) SetupBilling(ctx context.Context) error {
	_, err := c.c.SetupBilling(ctx, &pb.SetupBillingRequest{})
	return err
}

// GetBillingSession returns a billing portal session url.
func (c *Client) GetBillingSession(ctx context.Context) (*pb.GetBillingSessionResponse, error) {
	return c.c.GetBillingSession(ctx, &pb.GetBillingSessionRequest{})
}

// ListBillingUsers returns a list of users the account is responsible for.
func (c *Client) ListBillingUsers(ctx context.Context, opts ...ListOption) (
	*pb.ListBillingUsersResponse, error) {
	args := &listOptions{}
	for _, opt := range opts {
		opt(args)
	}
	return c.c.ListBillingUsers(ctx, &pb.ListBillingUsersRequest{
		Offset: args.offset,
		Limit:  args.limit,
	})
}

// IsUsernameAvailable returns a nil error if the username is valid and available.
func (c *Client) IsUsernameAvailable(ctx context.Context, username string) error {
	_, err := c.c.IsUsernameAvailable(ctx, &pb.IsUsernameAvailableRequest{
		Username: username,
	})
	return err
}

// IsOrgNameAvailable returns a nil error if the name is valid and available.
func (c *Client) IsOrgNameAvailable(ctx context.Context, name string) (*pb.IsOrgNameAvailableResponse, error) {
	return c.c.IsOrgNameAvailable(ctx, &pb.IsOrgNameAvailableRequest{
		Name: name,
	})
}

// DestroyAccount completely deletes an account and all associated data.
func (c *Client) DestroyAccount(ctx context.Context) error {
	_, err := c.c.DestroyAccount(ctx, &pb.DestroyAccountRequest{})
	return err
}
