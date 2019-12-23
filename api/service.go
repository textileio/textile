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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	loginTimeout = time.Minute * 3
	emailTimeout = time.Second * 10
)

// service is a gRPC service for textile.
type service struct {
	users    *c.Users
	sessions *c.Sessions
	teams    *c.Teams
	projects *c.Projects

	gateway     *gateway.Gateway
	emailClient *email.Client

	sessionSecret []byte
}

// Login handles a login request.
func (s *service) Login(req *pb.LoginRequest, stream pb.API_LoginServer) error {
	log.Debugf("received login request")

	matches, err := s.users.GetByEmail(req.Email)
	if err != nil {
		return err
	}
	var user *c.User
	// @todo: can we ensure in threads that a model never >1 by field?
	if len(matches) == 0 {
		user = &c.User{Email: req.Email}
		if err := s.users.Create(user); err != nil {
			return err
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
			return err
		}
		verification = uid.String()
	}

	// Send challenge email
	link := fmt.Sprintf("%s/verify/%s", s.gateway.Url(), verification)
	ctx, cancel := context.WithTimeout(context.Background(), emailTimeout)
	defer cancel()
	err = s.emailClient.VerifyAddress(ctx, user.Email, link)
	if err != nil {
		return err
	}

	if !s.awaitVerification(verification) {
		return fmt.Errorf("email not verified")
	}

	user.Token, err = generateAuthToken()
	if err != nil {
		return err
	}
	if err := s.users.Update(user); err != nil {
		return err
	}

	reply := &pb.LoginReply{
		ID:    user.ID,
		Token: user.Token,
	}
	return stream.Send(reply)
}

// AddTeam handles an add team request.
func (s *service) AddTeam(ctx context.Context, req *pb.AddTeamRequest) (*pb.AddTeamReply, error) {
	log.Debugf("received add team request")

	// @todo: Session middlewear
	// 1. get session from token
	// 2. inflate user from session
	// 3. check user is or is part of scope

	// 1. set team owner to session user
	// 1. update user team list

	team := &c.Team{
		Name: req.Name,
	}
	if err := s.teams.Create(team); err != nil {
		return nil, err
	}

	return &pb.AddTeamReply{
		ID: team.ID,
	}, nil
}

// AddProject handles an add project request.
func (s *service) AddProject(ctx context.Context, req *pb.AddProjectRequest) (*pb.AddProjectReply, error) {
	log.Debugf("received add project request")

	var scopeID string
	if req.ScopeID == "" {
		// @todo: look up user from session
	} else {
		user, err := s.users.Get(req.ScopeID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		scopeID = user.ID
	}

	proj := &c.Project{
		Name:    req.Name,
		ScopeID: scopeID,
	}
	if err := s.projects.Create(proj); err != nil {
		return nil, err
	}

	return &pb.AddProjectReply{
		ID:      proj.ID,
		StoreID: proj.StoreID,
	}, nil
}

// awaitVerification waits for a user to verify their email via a sent email.
func (s *service) awaitVerification(secret string) bool {
	listen := s.gateway.SessionListener()
	ch := make(chan struct{}, 1)
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

// @todo: finalize auth token design
func generateAuthToken() (token string, err error) {
	uid, err := uuid.NewRandom()
	if err != nil {
		return
	}
	return uid.String(), nil
}
