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

// AddAppToken add a new app token under the given project.
func (c *Client) AddAppToken(ctx context.Context, projID string, auth Auth) (*pb.AddAppTokenReply, error) {
	return c.c.AddAppToken(authCtx(ctx, auth), &pb.AddAppTokenRequest{
		ProjectID: projID,
	})
}

// ListAppTokens returns a list of all app tokens for the given project.
func (c *Client) ListAppTokens(ctx context.Context, projID string, auth Auth) (*pb.ListAppTokensReply, error) {
	return c.c.ListAppTokens(authCtx(ctx, auth), &pb.ListAppTokensRequest{
		ProjectID: projID,
	})
}

// RemoveAppToken removes an app token by ID.
func (c *Client) RemoveAppToken(ctx context.Context, tokenID string, auth Auth) error {
	_, err := c.c.RemoveAppToken(authCtx(ctx, auth), &pb.RemoveAppTokenRequest{
		ID: tokenID,
	})
	return err
}

// AddFolder adds a folder by name.
func (c *Client) AddFolder(ctx context.Context, projID, name string, public bool, auth Auth) (*pb.AddFolderReply, error) {
	return c.c.AddFolder(authCtx(ctx, auth), &pb.AddFolderRequest{
		Name:      name,
		Public:    public,
		ProjectID: projID,
	})
}

// GetFolder returns a folder by name.
func (c *Client) GetFolder(ctx context.Context, name string, auth Auth) (*pb.GetFolderReply, error) {
	return c.c.GetFolder(authCtx(ctx, auth), &pb.GetFolderRequest{
		Name: name,
	})
}

// ListFolders returns a list of folders under the current project.
func (c *Client) ListFolders(ctx context.Context, projID string, auth Auth) (*pb.ListFoldersReply, error) {
	return c.c.ListFolders(authCtx(ctx, auth), &pb.ListFoldersRequest{
		ProjectID: projID,
	})
}

// RemoveFolder removes a folder by name.
func (c *Client) RemoveFolder(ctx context.Context, name string, auth Auth) error {
	_, err := c.c.RemoveFolder(authCtx(ctx, auth), &pb.RemoveFolderRequest{
		Name: name,
	})
	return err
}

// AddFileOptions defines options for adding a file.
type AddFileOptions struct {
	Name     string
	Progress chan<- int64
}

// AddFileOption specifies an option for adding a file.
type AddFileOption func(*AddFileOptions)

// AddWithName adds a file with the given name.
func AddWithName(name string) AddFileOption {
	return func(args *AddFileOptions) {
		args.Name = name
	}
}

// AddWithProgress writes progress updates to the given channel.
func AddWithProgress(ch chan<- int64) AddFileOption {
	return func(args *AddFileOptions) {
		args.Progress = ch
	}
}

type addFileResult struct {
	path path.Resolved
	err  error
}

// AddFile uploads a file to the project store.
func (c *Client) AddFile(
	ctx context.Context,
	folder string,
	reader io.Reader,
	auth Auth,
	opts ...AddFileOption,
) (path.Resolved, error) {
	args := &AddFileOptions{}
	for _, opt := range opts {
		opt(args)
	}

	stream, err := c.c.AddFile(authCtx(ctx, auth))
	if err != nil {
		return nil, err
	}
	if err = stream.Send(&pb.AddFileRequest{
		Payload: &pb.AddFileRequest_Header_{
			Header: &pb.AddFileRequest_Header{
				Name:   args.Name,
				Folder: folder,
			},
		},
	}); err != nil {
		return nil, err
	}

	waitCh := make(chan addFileResult)
	go func() {
		defer close(waitCh)
		for {
			rep, err := stream.Recv()
			if err == io.EOF {
				return
			} else if err != nil {
				waitCh <- addFileResult{err: err}
				return
			}
			switch payload := rep.Payload.(type) {
			case *pb.AddFileReply_Event_:
				if payload.Event.Path != "" {
					id, err := cid.Parse(payload.Event.Path)
					if err != nil {
						waitCh <- addFileResult{err: err}
						return
					}
					waitCh <- addFileResult{path: path.IpfsPath(id)}
				} else if args.Progress != nil {
					select {
					case args.Progress <- payload.Event.Bytes:
					default:
					}
				}
			case *pb.AddFileReply_Error:
				waitCh <- addFileResult{err: fmt.Errorf(payload.Error)}
				return
			default:
				waitCh <- addFileResult{err: fmt.Errorf("invalid reply")}
				return
			}
		}
	}()

	buf := make([]byte, chunkSize)
	for {
		n, err := reader.Read(buf)
		if err == io.EOF {
			break
		} else if err != nil {
			_ = stream.CloseSend()
			return nil, err
		}
		if err = stream.Send(&pb.AddFileRequest{
			Payload: &pb.AddFileRequest_Chunk{
				Chunk: buf[:n],
			},
		}); err == io.EOF {
			break
		} else if err != nil {
			_ = stream.CloseSend()
			return nil, err
		}
	}
	if err = stream.CloseSend(); err != nil {
		return nil, err
	}
	res := <-waitCh
	return res.path, res.err
}

// GetFileOptions defines options for getting a file.
type GetFileOptions struct {
	Progress chan<- int64
}

// GetFileOption specifies an option for getting a file.
type GetFileOption func(*GetFileOptions)

// GetWithProgress writes progress updates to the given channel.
func GetWithProgress(ch chan<- int64) GetFileOption {
	return func(args *GetFileOptions) {
		args.Progress = ch
	}
}

// GetFile returns a file by its path.
func (c *Client) GetFile(
	ctx context.Context,
	folder string,
	pth path.Path,
	writer io.Writer,
	auth Auth,
	opts ...GetFileOption,
) error {
	args := &GetFileOptions{}
	for _, opt := range opts {
		opt(args)
	}

	stream, err := c.c.GetFile(authCtx(ctx, auth), &pb.GetFileRequest{
		Path:   pth.String(),
		Folder: folder,
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
			select {
			case args.Progress <- written:
			default:
			}
		}
	}
	return nil
}

// RemoveFile removes a file from a folder by name.
func (c *Client) RemoveFile(ctx context.Context, folder, name string, auth Auth) error {
	_, err := c.c.RemoveFile(authCtx(ctx, auth), &pb.RemoveFileRequest{
		Name:   name,
		Folder: folder,
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
		md["Authorization"] = "Bearer " + token
	}
	scope, ok := ctx.Value(authKey("scope")).(string)
	if ok && scope != "" {
		md["X-Scope"] = scope
	}
	return md, nil
}

func (t tokenAuth) RequireTransportSecurity() bool {
	return t.secure
}
