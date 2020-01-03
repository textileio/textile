package api

import (
	"context"
	"net/mail"
	"time"

	"github.com/google/uuid"
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

// service is a gRPC service for textile.
type service struct {
	collections *c.Collections

	gateway     *gateway.Gateway
	emailClient *email.Client

	sessionSecret []byte
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
	if s.sessionSecret != nil {
		secret = string(s.sessionSecret)
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
		ID:    user.ID,
		Token: session.ID,
	}, nil
}

// Switch handles a switch request.
func (s *service) Switch(ctx context.Context, req *pb.SwitchRequest) (*pb.SwitchReply, error) {
	log.Debugf("received switch request")

	session, ok := ctx.Value(reqKey("session")).(*c.Session)
	if !ok {
		log.Fatal("session required")
	}
	scope, ok := ctx.Value(reqKey("scope")).(string)
	if !ok {
		log.Fatal("scope required")
	}
	if err := s.collections.Sessions.SwitchScope(ctx, session, scope); err != nil {
		return nil, err
	}

	return &pb.SwitchReply{}, nil
}

// Logout handles a logout request.
func (s *service) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutReply, error) {
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
func (s *service) Whoami(ctx context.Context, req *pb.WhoamiRequest) (*pb.WhoamiReply, error) {
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

// ListTeams handles a list teams request.
func (s *service) ListTeams(ctx context.Context, req *pb.ListTeamsRequest) (*pb.ListTeamsReply, error) {
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

func teamToPbTeam(team *c.Team, members []*pb.GetTeamReply_Member) *pb.GetTeamReply {
	return &pb.GetTeamReply{
		ID:      team.ID,
		OwnerID: team.OwnerID,
		Name:    team.Name,
		Created: team.Created,
		Members: members,
	}
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

	proj, err := s.collections.Projects.Create(ctx, req.Name, scope)
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

	return projectToPbProject(proj), nil
}

// ListProjects handles a list projects request.
func (s *service) ListProjects(ctx context.Context, req *pb.ListProjectsRequest) (*pb.ListProjectsReply, error) {
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

func projectToPbProject(proj *c.Project) *pb.GetProjectReply {
	return &pb.GetProjectReply{
		ID:      proj.ID,
		Name:    proj.Name,
		StoreID: proj.StoreID,
		Created: proj.Created,
	}
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

// getTeamForUser returns a team if the user is authorized.
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

// getProjectForScope returns a project if the scope is authorized.
func (s *service) getProjectForScope(ctx context.Context, projID string, scope string) (*c.Project, error) {
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
