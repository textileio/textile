package api

import (
	"context"
	"fmt"

	pb "github.com/textileio/textile/api/pb"
	"github.com/textileio/textile/messaging"
	"github.com/textileio/textile/resources/users"
)

// service is a gRPC service for textile.
type service struct {
	users *users.Users
	email *messaging.EmailService
	//projects *projects.Projects
}

func (s *service) newUserChallenge(ctx context.Context, req *pb.LoginRequest, user *users.User) (*pb.LoginReply, error) {
	challenge := &users.Challenge{
		Shared: "ABC",
		Secret: "XYZ",
		Token:  "",
	}

	user.Challenge = challenge

	// send challenge email
	err := s.email.VerifyAddress(user.Email, fmt.Sprintf("https://service.link?email=%s&verification=%s", user.Email, challenge.Secret))
	if err != nil {
		return nil, err
	}

	if err := s.users.Create(user); err != nil {
		return nil, err
	}

	return &pb.LoginReply{
		ID:        user.ID,
		Challenge: challenge.Shared,
	}, nil
}

// Login handles a login request.
func (s *service) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
	log.Debugf("received login request")

	matches, err := s.users.GetByEmail(req.Email)
	if err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		// create new user
		user := &users.User{Email: req.Email}
		// create and send challenge
		return s.newUserChallenge(ctx, req, user)
	}

	user := matches[0]

	// user exists but is adding a new account
	if user.Challenge == nil {
		// create and send challenge
		return s.newUserChallenge(ctx, req, user)
	}

	// challenge issued but not completed
	if user.Challenge.Token == "" {
		return &pb.LoginReply{
			ID:        user.ID,
			Challenge: user.Challenge.Shared,
		}, nil
	}

	// challenge success so token exists
	token := user.Challenge.Token
	// move the new token to the user's token (empty array to drop all sessions)
	user.Tokens = append(user.Tokens, token)
	user.Challenge = nil

	if err := s.users.Create(user); err != nil {
		return nil, err
	}

	return &pb.LoginReply{
		ID:    user.ID,
		Token: token,
	}, nil
}
