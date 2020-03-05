package client

import (
	"context"
	"fmt"
	"io"

	"github.com/ipfs/go-cid"
	"github.com/ipfs/interface-go-ipfs-core/path"
	pb "github.com/textileio/textile/api/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	// chunkSize for add file requests.
	chunkSize = 1024
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
func NewClient(target string, creds credentials.TransportCredentials) (*Client, error) {
	var opts []grpc.DialOption
	auth := tokenAuth{}
	if creds != nil {
		opts = append(opts, grpc.WithTransportCredentials(creds))
		auth.secure = true
	} else {
		opts = append(opts, grpc.WithInsecure())
	}
	opts = append(opts, grpc.WithPerRPCCredentials(auth))
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
func (c *Client) Login(ctx context.Context, email string) (*pb.LoginReply, error) {
	return c.c.Login(ctx, &pb.LoginRequest{Email: email})
}

// Switch changes session scope.
func (c *Client) Switch(ctx context.Context, auth Auth) error {
	_, err := c.c.Switch(authCtx(ctx, auth), &pb.SwitchRequest{})
	return err
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
func (c *Client) AddProject(ctx context.Context, name string, auth Auth) (*pb.GetProjectReply, error) {
	return c.c.AddProject(authCtx(ctx, auth), &pb.AddProjectRequest{
		Name: name,
	})
}

// GetProject returns a project by name.
func (c *Client) GetProject(ctx context.Context, name string, auth Auth) (*pb.GetProjectReply, error) {
	return c.c.GetProject(authCtx(ctx, auth), &pb.GetProjectRequest{
		Name: name,
	})
}

// ListProjects returns a list of all authorized projects.
func (c *Client) ListProjects(ctx context.Context, auth Auth) (*pb.ListProjectsReply, error) {
	return c.c.ListProjects(authCtx(ctx, auth), &pb.ListProjectsRequest{})
}

// RemoveProject removes a project by name.
func (c *Client) RemoveProject(ctx context.Context, name string, auth Auth) error {
	_, err := c.c.RemoveProject(authCtx(ctx, auth), &pb.RemoveProjectRequest{
		Name: name,
	})
	return err
}

// AddToken add a new app token under the given project.
func (c *Client) AddToken(ctx context.Context, project string, auth Auth) (*pb.AddTokenReply, error) {
	return c.c.AddToken(authCtx(ctx, auth), &pb.AddTokenRequest{
		Project: project,
	})
}

// ListTokens returns a list of all app tokens for the given project.
func (c *Client) ListTokens(ctx context.Context, project string, auth Auth) (*pb.ListTokensReply, error) {
	return c.c.ListTokens(authCtx(ctx, auth), &pb.ListTokensRequest{
		Project: project,
	})
}

// RemoveToken removes an app token by ID.
func (c *Client) RemoveToken(ctx context.Context, tokenID string, auth Auth) error {
	_, err := c.c.RemoveToken(authCtx(ctx, auth), &pb.RemoveTokenRequest{
		ID: tokenID,
	})
	return err
}

// ListBucketPath returns information about a bucket path.
func (c *Client) ListBucketPath(ctx context.Context, project, pth string, auth Auth) (*pb.ListBucketPathReply, error) {
	return c.c.ListBucketPath(authCtx(ctx, auth), &pb.ListBucketPathRequest{
		Project: project,
		Path:    pth,
	})
}

// PushBucketPathOptions defines options for pushing a bucket path.
type PushBucketPathOptions struct {
	Progress chan<- int64
}

// PushBucketPathOption specifies an option for pushing a bucket path.
type PushBucketPathOption func(*PushBucketPathOptions)

// WithPushProgress writes progress updates to the given channel.
func WithPushProgress(ch chan<- int64) PushBucketPathOption {
	return func(args *PushBucketPathOptions) {
		args.Progress = ch
	}
}

type pushBucketPathResult struct {
	path path.Resolved
	root path.Path
	err  error
}

// PushBucketPath pushes a file to a bucket path.
// The bucket and any directory paths will be created if they don't exist.
// This will return the resolved path and the bucket's new root path.
func (c *Client) PushBucketPath(
	ctx context.Context,
	project, bucketPath string,
	reader io.Reader,
	auth Auth,
	opts ...PushBucketPathOption,
) (result path.Resolved, root path.Path, err error) {
	args := &PushBucketPathOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if args.Progress != nil {
		defer close(args.Progress)
	}

	stream, err := c.c.PushBucketPath(authCtx(ctx, auth))
	if err != nil {
		return nil, nil, err
	}
	if err = stream.Send(&pb.PushBucketPathRequest{
		Payload: &pb.PushBucketPathRequest_Header_{
			Header: &pb.PushBucketPathRequest_Header{
				Project: project,
				Path:    bucketPath,
			},
		},
	}); err != nil {
		return nil, nil, err
	}

	waitCh := make(chan pushBucketPathResult)
	go func() {
		defer close(waitCh)
		for {
			rep, err := stream.Recv()
			if err == io.EOF {
				return
			} else if err != nil {
				waitCh <- pushBucketPathResult{err: err}
				return
			}
			switch payload := rep.Payload.(type) {
			case *pb.PushBucketPathReply_Event_:
				if payload.Event.Path != "" {
					id, err := cid.Parse(payload.Event.Path)
					if err != nil {
						waitCh <- pushBucketPathResult{err: err}
						return
					}
					waitCh <- pushBucketPathResult{
						path: path.IpfsPath(id),
						root: path.New(payload.Event.Root.Path),
					}
				} else if args.Progress != nil {
					args.Progress <- payload.Event.Bytes
				}
			case *pb.PushBucketPathReply_Error:
				waitCh <- pushBucketPathResult{err: fmt.Errorf(payload.Error)}
				return
			default:
				waitCh <- pushBucketPathResult{err: fmt.Errorf("invalid reply")}
				return
			}
		}
	}()

	buf := make([]byte, chunkSize)
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if err := stream.Send(&pb.PushBucketPathRequest{
				Payload: &pb.PushBucketPathRequest_Chunk{
					Chunk: buf[:n],
				},
			}); err == io.EOF {
				break
			} else if err != nil {
				_ = stream.CloseSend()
				return nil, nil, err
			}
		}
		if err == io.EOF {
			break
		} else if err != nil {
			_ = stream.CloseSend()
			return nil, nil, err
		}
	}
	if err = stream.CloseSend(); err != nil {
		return nil, nil, err
	}
	res := <-waitCh
	return res.path, res.root, res.err
}

// PullBucketPathOptions defines options for pulling a bucket path.
type PullBucketPathOptions struct {
	Progress chan<- int64
}

// PullBucketPathOption specifies an option for pulling a bucket path.
type PullBucketPathOption func(*PullBucketPathOptions)

// WithPullProgress writes progress updates to the given channel.
func WithPullProgress(ch chan<- int64) PullBucketPathOption {
	return func(args *PullBucketPathOptions) {
		args.Progress = ch
	}
}

// PullBucketPath pulls the bucket path, writing it to writer if it's a file.
func (c *Client) PullBucketPath(
	ctx context.Context,
	bucketPath string,
	writer io.Writer,
	auth Auth,
	opts ...PullBucketPathOption,
) error {
	args := &PullBucketPathOptions{}
	for _, opt := range opts {
		opt(args)
	}
	if args.Progress != nil {
		defer close(args.Progress)
	}

	stream, err := c.c.PullBucketPath(authCtx(ctx, auth), &pb.PullBucketPathRequest{
		Path: bucketPath,
	})
	if err != nil {
		return err
	}

	var written int64
	for {
		rep, err := stream.Recv()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		n, err := writer.Write(rep.Chunk)
		if err != nil {
			return err
		}
		written += int64(n)
		if args.Progress != nil {
			args.Progress <- written
		}
	}
	return nil
}

// RemoveBucketPath removes the file or directory at path.
// Bucket files and directories will be unpinned.
// If the resulting bucket is empty, it will also be removed.
func (c *Client) RemoveBucketPath(ctx context.Context, pth string, auth Auth) error {
	_, err := c.c.RemoveBucketPath(authCtx(ctx, auth), &pb.RemoveBucketPathRequest{
		Path: pth,
	})
	return err
}

type authKey string

func authCtx(ctx context.Context, auth Auth) context.Context {
	ctx = context.WithValue(ctx, authKey("token"), auth.Token)
	if auth.Scope != "" {
		ctx = context.WithValue(ctx, authKey("scope"), auth.Scope)
	}
	return ctx
}

type tokenAuth struct {
	secure bool
}

func (t tokenAuth) GetRequestMetadata(ctx context.Context, _ ...string) (map[string]string, error) {
	md := map[string]string{}
	token, ok := ctx.Value(authKey("token")).(string)
	if ok && token != "" {
		md["authorization"] = "bearer " + token
	}
	scope, ok := ctx.Value(authKey("scope")).(string)
	if ok && scope != "" {
		md["x-scope"] = scope
	}
	return md, nil
}

func (t tokenAuth) RequireTransportSecurity() bool {
	return t.secure
}
