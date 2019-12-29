package client

import (
	"context"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/textileio/go-threads/util"
	pb "github.com/textileio/textile/api/pb"
	"google.golang.org/grpc"
)

// Auth is used to supply the client with authorization credentials.
type Auth struct {
	Token string
	Scope string // user or team ID
}

// Client provides the client api.
type Client struct {
	c    pb.APIClient
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
func (c *Client) Login(ctx context.Context, email string) (*pb.LoginReply, error) {
	return c.c.Login(ctx, &pb.LoginRequest{Email: email})
}

// AddTeam add a new team.
func (c *Client) AddTeam(ctx context.Context, name string, auth Auth) (*pb.AddTeamReply, error) {
	return c.c.AddTeam(authCtx(ctx, auth), &pb.AddTeamRequest{Name: name})
}

// GetTeam returns a team by ID.
func (c *Client) GetTeam(ctx context.Context, teamID string, auth Auth) (*pb.GetTeamReply, error) {
	return c.c.GetTeam(authCtx(ctx, auth), &pb.GetTeamRequest{
		ID: teamID,
	})
}

// ListTeams returns a list of authorized teams.
func (c *Client) ListTeams(ctx context.Context, auth Auth) (*pb.ListTeamsReply, error) {
	return c.c.ListTeams(authCtx(ctx, auth), &pb.ListTeamsRequest{})
}

// RemoveTeam removes a team by ID.
func (c *Client) RemoveTeam(ctx context.Context, teamID string, auth Auth) error {
	_, err := c.c.RemoveTeam(authCtx(ctx, auth), &pb.RemoveTeamRequest{
		ID: teamID,
	})
	return err
}

// InviteToTeam invites the given email to a team by ID.
func (c *Client) InviteToTeam(ctx context.Context, teamID, email string, auth Auth) (*pb.InviteToTeamReply, error) {
	return c.c.InviteToTeam(authCtx(ctx, auth), &pb.InviteToTeamRequest{
		ID:    teamID,
		Email: email,
	})
}

// LeaveTeam removes the authorized user from a team by ID.
func (c *Client) LeaveTeam(ctx context.Context, teamID string, auth Auth) error {
	_, err := c.c.LeaveTeam(authCtx(ctx, auth), &pb.LeaveTeamRequest{
		ID: teamID,
	})
	return err
}

// AddProject add a new project under the given scope.
func (c *Client) AddProject(ctx context.Context, name string, auth Auth) (*pb.AddProjectReply, error) {
	return c.c.AddProject(authCtx(ctx, auth), &pb.AddProjectRequest{
		Name: name,
	})
}

// GetProject returns a project by ID.
func (c *Client) GetProject(ctx context.Context, projID string, auth Auth) (*pb.GetProjectReply, error) {
	return c.c.GetProject(authCtx(ctx, auth), &pb.GetProjectRequest{
		ID: projID,
	})
}

// ListProjects returns a list of all authorized projects.
func (c *Client) ListProjects(ctx context.Context, auth Auth) (*pb.ListProjectsReply, error) {
	return c.c.ListProjects(authCtx(ctx, auth), &pb.ListProjectsRequest{})
}

// RemoveProject removes a project by ID.
func (c *Client) RemoveProject(ctx context.Context, projID string, auth Auth) error {
	_, err := c.c.RemoveProject(authCtx(ctx, auth), &pb.RemoveProjectRequest{
		ID: projID,
	})
	return err
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

func authCtx(ctx context.Context, auth Auth) context.Context {
	ctx = context.WithValue(ctx, authKey("token"), auth.Token)
	if auth.Scope != "" {
		ctx = context.WithValue(ctx, authKey("scope"), auth.Scope)
	}
	return ctx
}
