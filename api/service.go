package api

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	pb "github.com/textileio/textile/api/pb"
	c "github.com/textileio/textile/collections"
	"github.com/textileio/textile/email"
	"github.com/textileio/textile/gateway"
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

	matches, err := s.collections.Users.GetByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	var user *c.User
	if len(matches) == 0 {
		user, err = s.collections.Users.Create(req.Email)
		if err != nil {
			return nil, err
		}
	} else {
		user = matches[0]
	}

	var verification string
	if s.sessionSecret != nil {
		verification = string(s.sessionSecret)
	} else {
		uid, err := uuid.NewRandom()
		if err != nil {
			return nil, err
		}
		verification = uid.String()
	}

	// Send challenge email
	link := fmt.Sprintf("%s/verify/%s", s.gateway.Url(), verification)
	ectx, cancel := context.WithTimeout(ctx, emailTimeout)
	defer cancel()
	if err = s.emailClient.VerifyAddress(ectx, user.Email, link); err != nil {
		return nil, err
	}

	if !s.awaitVerification(verification) {
		return nil, fmt.Errorf("email not verified")
	}

	session, err := s.collections.Sessions.Create(user.ID)
	if err != nil {
		return nil, err
	}

	return &pb.LoginReply{
		ID:    user.ID,
		Token: session.ID,
	}, nil
}

// AddTeam handles an add team request.
func (s *service) AddTeam(ctx context.Context, req *pb.AddTeamRequest) (*pb.AddTeamReply, error) {
	log.Debugf("received add team request")

	user, ok := ctx.Value(reqKey("user")).(*c.User)
	if !ok {
		log.Fatal("user required")
	}

	team := &c.Team{
		Name:    req.Name,
		OwnerID: user.ID,
	}
	if err := s.collections.Teams.Create(team); err != nil {
		return nil, err
	}

	if err := s.collections.Users.AddTeam(user, team); err != nil {
		return nil, err
	}

	return &pb.AddTeamReply{
		ID: team.ID,
	}, nil
}

// GetTeam handles a get team request.
func (s *service) GetTeam(ctx context.Context, req *pb.GetTeamRequest) (*pb.GetTeamReply, error) {
	panic("implement me")
}

// ListTeams handles a list teams request.
func (s *service) ListTeams(ctx context.Context, req *pb.ListTeamsRequest) (*pb.ListTeamsReply, error) {
	panic("implement me")
}

// RemoveTeam handles a remove team request.
func (s *service) RemoveTeam(ctx context.Context, req *pb.RemoveTeamRequest) (*pb.RemoveTeamReply, error) {
	panic("implement me")
}

// LeaveTeam handles a leave team request.
func (s *service) LeaveTeam(ctx context.Context, req *pb.LeaveTeamRequest) (*pb.LeaveTeamReply, error) {
	panic("implement me")
}

// InviteToTeam handles a team invite request.
func (s *service) InviteToTeam(ctx context.Context, req *pb.InviteToTeamRequest) (*pb.InviteToTeamReply, error) {
	panic("implement me")
}

// AddProject handles an add project request.
func (s *service) AddProject(ctx context.Context, req *pb.AddProjectRequest) (*pb.AddProjectReply, error) {
	log.Debugf("received add project request")

	user, ok := ctx.Value(reqKey("user")).(*c.User)
	if !ok {
		log.Fatal("user required")
	}

	proj := &c.Project{Name: req.Name}

	team, ok := ctx.Value(reqKey("team")).(*c.Team)
	if ok {
		proj.Scope = team.ID
	} else {
		proj.Scope = user.ID
	}

	if err := s.collections.Projects.Create(proj); err != nil {
		return nil, err
	}

	return &pb.AddProjectReply{
		ID:      proj.ID,
		StoreID: proj.StoreID,
	}, nil
}

// GetProject handles a get project request.
func (s *service) GetProject(ctx context.Context, req *pb.GetProjectRequest) (*pb.GetProjectReply, error) {
	panic("implement me")
}

// ListProjects handles a list projects request.
func (s *service) ListProjects(ctx context.Context, req *pb.ListProjectsRequest) (*pb.ListProjectsReply, error) {
	panic("implement me")
}

// RemoveProject handles a remove project request.
func (s *service) RemoveProject(ctx context.Context, req *pb.RemoveProjectRequest) (*pb.RemoveProjectReply, error) {
	panic("implement me")
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
