package api

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/mail"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	ipfsfiles "github.com/ipfs/go-ipfs-files"
	iface "github.com/ipfs/interface-go-ipfs-core"
	"github.com/ipfs/interface-go-ipfs-core/options"
	"github.com/ipfs/interface-go-ipfs-core/path"
	fc "github.com/textileio/filecoin/api/client"
	pb "github.com/textileio/textile/api/pb"
	c "github.com/textileio/textile/collections"
	"github.com/textileio/textile/email"
	"github.com/textileio/textile/gateway"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	loginTimeout = time.Minute * 3
	emailTimeout = time.Second * 10
)

const (
	// bucketSeedName is the name of the seed file used to ensure buckets are unique
	bucketSeedName = ".textilebucketseed"
	// chunkSize for get file requests.
	chunkSize = 1024 * 32
)

// service is a gRPC service for textile.
type service struct {
	collections *c.Collections

	gateway        *gateway.Gateway
	emailClient    *email.Client
	ipfsClient     iface.CoreAPI
	filecoinClient *fc.Client

	sessionSecret string
}

// Login handles a login request.
func (s *service) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
	log.Debugf("received login request")

	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Email address in not valid")
	}

	matches, err := s.collections.Users.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	var user *c.User
	if len(matches) == 0 {
		user, err = s.collections.Users.Create(ctx, req.Email)
		if err != nil {
			return nil, err
		}
	} else {
		user = matches[0]
	}

	var secret string
	if s.sessionSecret != "" {
		secret = s.sessionSecret
	} else {
		uid, err := uuid.NewRandom()
		if err != nil {
			return nil, err
		}
		secret = uid.String()
	}

	ectx, cancel := context.WithTimeout(ctx, emailTimeout)
	defer cancel()
	if err = s.emailClient.ConfirmAddress(ectx, user.Email, s.gateway.Url(), secret); err != nil {
		return nil, err
	}

	if !s.awaitVerification(secret) {
		return nil, status.Error(codes.Unauthenticated, "Could not verify email address")
	}

	session, err := s.collections.Sessions.Create(ctx, user.ID, user.ID)
	if err != nil {
		return nil, err
	}

	return &pb.LoginReply{
		ID:        user.ID,
		SessionID: session.ID,
	}, nil
}

// Switch handles a switch request.
func (s *service) Switch(ctx context.Context, _ *pb.SwitchRequest) (*pb.SwitchReply, error) {
	log.Debugf("received switch request")

	session, ok := ctx.Value(reqKey("session")).(*c.Session)
	if !ok {
		log.Fatal("session required")
	}
	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	session.Scope = scope
	if err := s.collections.Sessions.Save(ctx, session); err != nil {
		return nil, err
	}

	return &pb.SwitchReply{}, nil
}

// Logout handles a logout request.
func (s *service) Logout(ctx context.Context, _ *pb.LogoutRequest) (*pb.LogoutReply, error) {
	log.Debugf("received logout request")

	session, ok := ctx.Value(reqKey("session")).(*c.Session)
	if !ok {
		log.Fatal("session required")
	}
	if err := s.collections.Sessions.Delete(ctx, session.ID); err != nil {
		return nil, err
	}

	return &pb.LogoutReply{}, nil
}

// Whoami handles a whoami request.
func (s *service) Whoami(ctx context.Context, _ *pb.WhoamiRequest) (*pb.WhoamiReply, error) {
	log.Debugf("received whoami request")

	user, ok := ctx.Value(reqKey("user")).(*c.User)
	if !ok {
		log.Fatal("user required")
	}
	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}

	reply := &pb.WhoamiReply{
		ID:    user.ID,
		Email: user.Email,
	}
	if scope != user.ID {
		team, err := s.collections.Teams.Get(ctx, scope)
		if err != nil {
			return nil, err
		}
		reply.TeamID = team.ID
		reply.TeamName = team.Name
	}

	return reply, nil
}

// awaitVerification waits for a user to verify their email via a sent email.
func (s *service) awaitVerification(secret string) bool {
	listen := s.gateway.SessionListener()
	ch := make(chan struct{})
	timer := time.NewTimer(loginTimeout)
	go func() {
		for i := range listen.Channel() {
			if r, ok := i.(string); ok && r == secret {
				ch <- struct{}{}
			}
		}
	}()
	select {
	case <-ch:
		listen.Discard()
		timer.Stop()
		return true
	case <-timer.C:
		listen.Discard()
		return false
	}
}

// AddTeam handles an add team request.
func (s *service) AddTeam(ctx context.Context, req *pb.AddTeamRequest) (*pb.AddTeamReply, error) {
	log.Debugf("received add team request")

	user, ok := ctx.Value(reqKey("user")).(*c.User)
	if !ok {
		log.Fatal("user required")
	}

	team, err := s.collections.Teams.Create(ctx, user.ID, req.Name)
	if err != nil {
		return nil, err
	}
	if err = s.collections.Users.JoinTeam(ctx, user, team.ID); err != nil {
		return nil, err
	}

	return &pb.AddTeamReply{
		ID: team.ID,
	}, nil
}

// GetTeam handles a get team request.
func (s *service) GetTeam(ctx context.Context, req *pb.GetTeamRequest) (*pb.GetTeamReply, error) {
	log.Debugf("received get team request")

	user, ok := ctx.Value(reqKey("user")).(*c.User)
	if !ok {
		log.Fatal("user required")
	}
	team, err := s.getTeamForUser(ctx, req.ID, user)
	if err != nil {
		return nil, err
	}

	users, err := s.collections.Users.ListByTeam(ctx, team.ID)
	if err != nil {
		return nil, err
	}
	members := make([]*pb.GetTeamReply_Member, len(users))
	for i, u := range users {
		members[i] = &pb.GetTeamReply_Member{
			ID:    u.ID,
			Email: u.Email,
		}
	}

	return teamToPbTeam(team, members), nil
}

func (s *service) getTeamForUser(ctx context.Context, teamID string, user *c.User) (*c.Team, error) {
	team, err := s.collections.Teams.Get(ctx, teamID)
	if err != nil {
		return nil, err
	}
	if team == nil {
		return nil, status.Error(codes.NotFound, "Team not found")
	}
	if !s.collections.Users.HasTeam(user, team.ID) {
		return nil, status.Error(codes.PermissionDenied, "User is not a team member")
	}
	return team, nil
}

func teamToPbTeam(team *c.Team, members []*pb.GetTeamReply_Member) *pb.GetTeamReply {
	return &pb.GetTeamReply{
		ID:      team.ID,
		OwnerID: team.OwnerID,
		Name:    team.Name,
		Created: team.Created,
		Members: members,
	}
}

// ListTeams handles a list teams request.
func (s *service) ListTeams(ctx context.Context, _ *pb.ListTeamsRequest) (*pb.ListTeamsReply, error) {
	log.Debugf("received list teams request")

	user, ok := ctx.Value(reqKey("user")).(*c.User)
	if !ok {
		log.Fatal("user required")
	}

	list := make([]*pb.GetTeamReply, len(user.Teams))
	for i, id := range user.Teams {
		team, err := s.collections.Teams.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		list[i] = teamToPbTeam(team, nil) // don't inflate members
	}

	return &pb.ListTeamsReply{List: list}, nil
}

// RemoveTeam handles a remove team request.
// @todo: Delete team projects.
func (s *service) RemoveTeam(ctx context.Context, req *pb.RemoveTeamRequest) (*pb.RemoveTeamReply, error) {
	log.Debugf("received remove team request")

	user, ok := ctx.Value(reqKey("user")).(*c.User)
	if !ok {
		log.Fatal("user required")
	}
	team, err := s.getTeamForUser(ctx, req.ID, user)
	if err != nil {
		return nil, err
	}
	if team.OwnerID != user.ID {
		return nil, status.Error(codes.PermissionDenied, "User is not the team owner")
	}

	if err = s.collections.Teams.Delete(ctx, team.ID); err != nil {
		return nil, err
	}
	users, err := s.collections.Users.ListByTeam(ctx, team.ID)
	if err != nil {
		return nil, err
	}
	for _, u := range users {
		if err = s.collections.Users.LeaveTeam(ctx, u, team.ID); err != nil {
			return nil, err
		}
	}

	return &pb.RemoveTeamReply{}, nil
}

// InviteToTeam handles a team invite request.
func (s *service) InviteToTeam(ctx context.Context, req *pb.InviteToTeamRequest) (*pb.InviteToTeamReply, error) {
	log.Debugf("received invite to team request")

	user, ok := ctx.Value(reqKey("user")).(*c.User)
	if !ok {
		log.Fatal("user required")
	}
	team, err := s.getTeamForUser(ctx, req.ID, user)
	if err != nil {
		return nil, err
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Email address in not valid")
	}

	invite, err := s.collections.Invites.Create(ctx, team.ID, user.ID, req.Email)
	if err != nil {
		return nil, err
	}

	ectx, cancel := context.WithTimeout(ctx, emailTimeout)
	defer cancel()
	if err = s.emailClient.InviteAddress(
		ectx, team.Name, user.Email, req.Email, s.gateway.Url(), invite.ID); err != nil {
		return nil, err
	}

	return &pb.InviteToTeamReply{InviteID: invite.ID}, nil
}

// LeaveTeam handles a leave team request.
func (s *service) LeaveTeam(ctx context.Context, req *pb.LeaveTeamRequest) (*pb.LeaveTeamReply, error) {
	log.Debugf("received leave team request")

	user, ok := ctx.Value(reqKey("user")).(*c.User)
	if !ok {
		log.Fatal("user required")
	}
	team, err := s.getTeamForUser(ctx, req.ID, user)
	if err != nil {
		return nil, err
	}
	if team.OwnerID == user.ID {
		return nil, status.Error(codes.PermissionDenied, "Team owner cannot leave")
	}

	if err = s.collections.Users.LeaveTeam(ctx, user, team.ID); err != nil {
		return nil, err
	}

	return &pb.LeaveTeamReply{}, nil
}

// AddProject handles an add project request.
func (s *service) AddProject(ctx context.Context, req *pb.AddProjectRequest) (*pb.AddProjectReply, error) {
	log.Debugf("received add project request")

	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}

	var addr string
	if s.filecoinClient != nil {
		var err error
		addr, err = s.filecoinClient.Wallet.NewWallet(ctx, "bls")
		if err != nil {
			return nil, err
		}
	}

	proj, err := s.collections.Projects.Create(ctx, req.Name, scope, addr)
	if err != nil {
		return nil, err
	}

	return &pb.AddProjectReply{
		ID:      proj.ID,
		StoreID: proj.StoreID,
	}, nil
}

// GetProject handles a get project request.
func (s *service) GetProject(ctx context.Context, req *pb.GetProjectRequest) (*pb.GetProjectReply, error) {
	log.Debugf("received get project request")

	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	proj, err := s.getProjectForScope(ctx, req.ID, scope)
	if err != nil {
		return nil, err
	}

	var bal int64
	if proj.WalletAddress != "" {
		bal, err = s.filecoinClient.Wallet.WalletBalance(ctx, proj.WalletAddress)
		if err != nil {
			return nil, err
		}
	}

	reply := projectToPbProject(proj)
	reply.WalletBalance = bal

	return reply, nil
}

func (s *service) getProjectForScope(ctx context.Context, projID, scope string) (*c.Project, error) {
	proj, err := s.collections.Projects.Get(ctx, projID)
	if err != nil {
		return nil, err
	}
	if proj == nil {
		return nil, status.Error(codes.NotFound, "Project not found")
	}
	if proj.Scope != scope {
		return nil, status.Error(codes.PermissionDenied, "Scope does not own project")
	}
	return proj, nil
}

func projectToPbProject(proj *c.Project) *pb.GetProjectReply {
	return &pb.GetProjectReply{
		ID:            proj.ID,
		Name:          proj.Name,
		StoreID:       proj.StoreID,
		WalletAddress: proj.WalletAddress,
		Created:       proj.Created,
	}
}

// ListProjects handles a list projects request.
func (s *service) ListProjects(ctx context.Context, _ *pb.ListProjectsRequest) (*pb.ListProjectsReply, error) {
	log.Debugf("received list projects request")

	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}

	projs, err := s.collections.Projects.List(ctx, scope)
	if err != nil {
		return nil, err
	}
	list := make([]*pb.GetProjectReply, len(projs))
	for i, proj := range projs {
		list[i] = projectToPbProject(proj)
	}

	return &pb.ListProjectsReply{List: list}, nil
}

// RemoveProject handles a remove project request.
func (s *service) RemoveProject(ctx context.Context, req *pb.RemoveProjectRequest) (*pb.RemoveProjectReply, error) {
	log.Debugf("received remove project request")

	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	proj, err := s.getProjectForScope(ctx, req.ID, scope)
	if err != nil {
		return nil, err
	}

	if err = s.collections.Projects.Delete(ctx, proj.ID); err != nil {
		return nil, err
	}

	return &pb.RemoveProjectReply{}, nil
}

// AddAppToken handles an add app token request.
func (s *service) AddAppToken(ctx context.Context, req *pb.AddAppTokenRequest) (*pb.AddAppTokenReply, error) {
	log.Debugf("received add app token request")

	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	proj, err := s.getProjectForScope(ctx, req.ProjectID, scope)
	if err != nil {
		return nil, err
	}
	token, err := s.collections.AppTokens.Create(ctx, proj.ID)
	if err != nil {
		return nil, err
	}

	return &pb.AddAppTokenReply{
		ID: token.ID,
	}, nil
}

// ListAppTokens handles a list app tokens request.
func (s *service) ListAppTokens(ctx context.Context, req *pb.ListAppTokensRequest) (*pb.ListAppTokensReply, error) {
	log.Debugf("received list app tokens request")

	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	proj, err := s.getProjectForScope(ctx, req.ProjectID, scope)
	if err != nil {
		return nil, err
	}

	tokens, err := s.collections.AppTokens.List(ctx, proj.ID)
	if err != nil {
		return nil, err
	}
	list := make([]string, len(tokens))
	for i, token := range tokens {
		list[i] = token.ID
	}

	return &pb.ListAppTokensReply{List: list}, nil
}

// RemoveAppToken handles a remove app token request.
func (s *service) RemoveAppToken(ctx context.Context, req *pb.RemoveAppTokenRequest) (*pb.RemoveAppTokenReply, error) {
	log.Debugf("received remove app token request")

	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	token, err := s.getAppTokenWithScope(ctx, req.ID, scope)
	if err != nil {
		return nil, err
	}

	if err = s.collections.AppTokens.Delete(ctx, token.ID); err != nil {
		return nil, err
	}

	return &pb.RemoveAppTokenReply{}, nil
}

func (s *service) getAppTokenWithScope(ctx context.Context, tokenID, scope string) (*c.AppToken, error) {
	token, err := s.collections.AppTokens.Get(ctx, tokenID)
	if err != nil {
		return nil, err
	}
	if token == nil {
		return nil, status.Error(codes.NotFound, "Token not found")
	}
	if _, err := s.getProjectForScope(ctx, token.ProjectID, scope); err != nil {
		return nil, err
	}
	return token, nil
}

// ListBucketPath handles a list bucket path request.
func (s *service) ListBucketPath(ctx context.Context, req *pb.ListBucketPathRequest) (*pb.ListBucketPathReply, error) {
	log.Debugf("received list bucket path request")

	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}

	if req.Path == "" { // List top-level buckets for this project
		proj, err := s.getProjectForScope(ctx, req.ProjectID, scope)
		if err != nil {
			return nil, err
		}
		bucks, err := s.collections.Buckets.List(ctx, proj.ID)
		if err != nil {
			return nil, err
		}
		items := make([]*pb.ListBucketPathReply_Item, len(bucks))
		for i, buck := range bucks {
			items[i], err = s.pathToBucketItem(ctx, path.New(buck.Path), false)
			if err != nil {
				return nil, err
			}
			items[i].Name = buck.Name
		}

		return &pb.ListBucketPathReply{
			Item: &pb.ListBucketPathReply_Item{
				IsDir: true,
				Items: items,
			},
		}, nil
	}

	buck, pth, err := s.getBucketAndPathWithScope(ctx, req.Path, scope)
	if err != nil {
		return nil, err
	}

	return s.bucketPathToPb(ctx, buck, pth, true)
}

func (s *service) bucketPathToPb(
	ctx context.Context,
	buck *c.Bucket,
	pth path.Path,
	followLinks bool,
) (*pb.ListBucketPathReply, error) {
	item, err := s.pathToBucketItem(ctx, pth, followLinks)
	if err != nil {
		return nil, err
	}

	rep := &pb.ListBucketPathReply{
		Item: item,
		Root: &pb.BucketRoot{
			Name:    buck.Name,
			Path:    buck.Path,
			Created: buck.Created,
			Updated: buck.Updated,
			Public:  buck.Public,
		},
	}

	return rep, nil
}

func (s *service) pathToBucketItem(ctx context.Context, pth path.Path, followLinks bool) (*pb.ListBucketPathReply_Item, error) {
	node, err := s.ipfsClient.Unixfs().Get(ctx, pth)
	if err != nil {
		return nil, err
	}
	defer node.Close()

	return nodeToBucketItem(pth.String(), node, followLinks)
}

func nodeToBucketItem(pth string, node ipfsfiles.Node, followLinks bool) (*pb.ListBucketPathReply_Item, error) {
	size, err := node.Size()
	if err != nil {
		return nil, err
	}
	item := &pb.ListBucketPathReply_Item{
		Name: filepath.Base(pth),
		Path: pth,
		Size: size,
	}
	switch node := node.(type) {
	case ipfsfiles.Directory:
		item.IsDir = true
		entries := node.Entries()
		for entries.Next() {
			if entries.Name() == bucketSeedName {
				continue
			}
			i := &pb.ListBucketPathReply_Item{}
			if followLinks {
				n := entries.Node()
				i, err = nodeToBucketItem(filepath.Join(pth, entries.Name()), n, false)
				if err != nil {
					n.Close()
					return nil, err
				}
				n.Close()
			}
			item.Items = append(item.Items, i)
		}
		if err := entries.Err(); err != nil {
			return nil, err
		}
	}
	return item, nil
}

func (s *service) getBucketAndPathWithScope(ctx context.Context, pth, scope string) (*c.Bucket, path.Path, error) {
	buckName, fileName, err := parsePath(pth)
	if err != nil {
		return nil, nil, err
	}
	buck, err := s.getBucketWithScope(ctx, buckName, scope)
	if err != nil {
		return nil, nil, err
	}
	npth := path.New(filepath.Join(buck.Path, fileName))
	if err = npth.IsValid(); err != nil {
		return nil, nil, err
	}
	return buck, npth, err
}

func parsePath(pth string) (folder, name string, err error) {
	if strings.Contains(pth, bucketSeedName) {
		err = fmt.Errorf("paths containing %s are not allowed", bucketSeedName)
		return
	}
	pth = strings.TrimPrefix(pth, "/")
	parts := strings.SplitN(pth, "/", 2)
	folder = parts[0]
	if len(parts) > 1 {
		name = parts[1]
	}
	return
}

func (s *service) getBucketWithScope(ctx context.Context, name, scope string) (*c.Bucket, error) {
	folder, err := s.collections.Buckets.GetByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if folder == nil {
		return nil, status.Error(codes.NotFound, "Bucket not found")
	}
	if _, err := s.getProjectForScope(ctx, folder.ProjectID, scope); err != nil {
		return nil, err
	}
	return folder, nil
}

// PushBucketPath handles a push bucket path request.
func (s *service) PushBucketPath(server pb.API_PushBucketPathServer) error {
	log.Debugf("received push bucket path request")

	scope, ok := server.Context().Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}

	req, err := server.Recv()
	if err != nil {
		return err
	}
	var projectID, filePath string
	switch payload := req.Payload.(type) {
	case *pb.PushBucketPathRequest_Header_:
		projectID = payload.Header.ProjectID
		filePath = payload.Header.Path
	default:
		return fmt.Errorf("push bucket path header is required")
	}
	buckName, fileName, err := parsePath(filePath)
	if err != nil {
		return err
	}
	buck, err := s.getBucketWithScope(server.Context(), buckName, scope)
	if err != nil {
		if status.Convert(err).Code() != codes.NotFound {
			return err
		}
		buck, err = s.createBucket(server.Context(), buckName, false, projectID)
		if err != nil {
			return err
		}
	}

	sendEvent := func(event *pb.PushBucketPathReply_Event) error {
		return server.Send(&pb.PushBucketPathReply{
			Payload: &pb.PushBucketPathReply_Event_{
				Event: event,
			},
		})
	}

	sendErr := func(err error) {
		if err := server.Send(&pb.PushBucketPathReply{
			Payload: &pb.PushBucketPathReply_Error{
				Error: err.Error(),
			},
		}); err != nil {
			log.Errorf("error sending error: %v", err)
		}
	}

	reader, writer := io.Pipe()
	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		for {
			req, err := server.Recv()
			if err == io.EOF {
				_ = writer.Close()
				return
			} else if err != nil {
				_ = writer.CloseWithError(err)
				sendErr(err)
				return
			}
			switch payload := req.Payload.(type) {
			case *pb.PushBucketPathRequest_Chunk:
				if _, err := writer.Write(payload.Chunk); err != nil {
					sendErr(err)
					return
				}
			default:
				sendErr(fmt.Errorf("invalid request"))
				return
			}
		}
	}()

	var size string
	eventCh := make(chan interface{})
	defer close(eventCh)
	go func() {
		for e := range eventCh {
			event, ok := e.(*iface.AddEvent)
			if !ok {
				log.Error("unexpected event type")
				continue
			}
			if event.Path == nil { // This is a progress event
				if err := sendEvent(&pb.PushBucketPathReply_Event{
					Name:  event.Name,
					Bytes: event.Bytes,
				}); err != nil {
					log.Errorf("error sending event: %v", err)
				}
			} else {
				size = event.Size // Save size for use in the final response
			}
		}
	}()

	pth, err := s.ipfsClient.Unixfs().Add(
		server.Context(),
		ipfsfiles.NewReaderFile(reader),
		options.Unixfs.Pin(false),
		options.Unixfs.Progress(true),
		options.Unixfs.Events(eventCh))
	if err != nil {
		return err
	}

	buckPath := path.New(buck.Path)
	dirpth, err := s.ipfsClient.Object().
		AddLink(server.Context(), buckPath, fileName, pth, options.Object.Create(true))
	if err != nil {
		return err
	}
	if err = s.ipfsClient.Pin().Update(server.Context(), buckPath, dirpth); err != nil {
		return err
	}

	buck.Path = dirpth.String()
	buck.Updated = time.Now().Unix()
	if err = s.collections.Buckets.Save(server.Context(), buck); err != nil {
		return err
	}

	if err = sendEvent(&pb.PushBucketPathReply_Event{
		Path: pth.String(),
		Size: size,
		Root: &pb.BucketRoot{
			Name:    buck.Name,
			Path:    buck.Path,
			Created: buck.Created,
			Updated: buck.Updated,
			Public:  false,
		},
	}); err != nil {
		return err
	}

	log.Debugf("pushed %s to bucket: %s", fileName, buck.Name)
	return nil
}

func (s *service) createBucket(ctx context.Context, name string, public bool, projectID string) (*c.Bucket, error) {
	seed := make([]byte, 32)
	_, err := rand.Read(seed)
	if err != nil {
		return nil, err
	}

	pth, err := s.ipfsClient.Unixfs().Add(
		ctx,
		ipfsfiles.NewMapDirectory(map[string]ipfsfiles.Node{
			bucketSeedName: ipfsfiles.NewBytesFile(seed),
		}),
		options.Unixfs.Pin(true))
	if err != nil {
		return nil, err
	}

	return s.collections.Buckets.Create(ctx, pth, name, public, projectID)
}

// PullBucketPath handles a pull bucket path request.
func (s *service) PullBucketPath(req *pb.PullBucketPathRequest, server pb.API_PullBucketPathServer) error {
	log.Debugf("received pull bucket path request")

	scope, ok := server.Context().Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	_, pth, err := s.getBucketAndPathWithScope(server.Context(), req.Path, scope)
	if err != nil {
		return err
	}
	node, err := s.ipfsClient.Unixfs().Get(server.Context(), pth)
	if err != nil {
		return err
	}
	defer node.Close()

	file := ipfsfiles.ToFile(node)
	if file == nil {
		return fmt.Errorf("node is a directory")
	}
	buf := make([]byte, chunkSize)
	for {
		n, err := file.Read(buf)
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}
		if err := server.Send(&pb.PullBucketPathReply{
			Chunk: buf[:n],
		}); err != nil {
			return err
		}
	}
	return nil
}

// RemoveBucketPath handles a remove bucket path request.
func (s *service) RemoveBucketPath(
	ctx context.Context,
	req *pb.RemoveBucketPathRequest,
) (*pb.RemoveBucketPathReply, error) {
	log.Debugf("received remove bucket path request")

	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	buckName, fileName, err := parsePath(req.Path)
	if err != nil {
		return nil, err
	}
	buck, err := s.getBucketWithScope(ctx, buckName, scope)
	if err != nil {
		return nil, err
	}

	buckPath := path.New(buck.Path)
	dirpth, err := s.ipfsClient.Object().RmLink(ctx, buckPath, fileName)
	if err != nil {
		return nil, err
	}

	links, err := s.ipfsClient.Unixfs().Ls(ctx, dirpth)
	if err != nil {
		return nil, err
	}

	var cnt int
	for range links {
		cnt++
	}
	if cnt > 1 { // Account for the seed file
		if err = s.ipfsClient.Pin().Update(ctx, buckPath, dirpth); err != nil {
			return nil, err
		}

		buck.Path = dirpth.String()
		buck.Updated = time.Now().Unix()
		if err = s.collections.Buckets.Save(ctx, buck); err != nil {
			return nil, err
		}
		log.Debugf("removed %s from bucket: %s", fileName, buck.Name)
	} else {
		if err = s.ipfsClient.Pin().Rm(ctx, buckPath); err != nil {
			return nil, err
		}

		if err = s.collections.Buckets.Delete(ctx, buck.ID); err != nil {
			return nil, err
		}
		log.Debugf("removed bucket: %s", buck.Name)
	}

	return &pb.RemoveBucketPathReply{}, nil
}
