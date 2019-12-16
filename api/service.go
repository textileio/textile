package api

import (
	"context"

	pb "github.com/textileio/textile/api/pb"
	"github.com/textileio/textile/resources/users"
)

// service is a gRPC service for textile.
type service struct {
	users *users.Users
	//projects *projects.Projects
}

// Login handles a login request.
func (s *service) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
	log.Debugf("received login request")

	user := &users.User{Email: req.Email}
	if err := s.users.Create(user); err != nil {
		return nil, err
	}

	return &pb.LoginReply{
		ID:    user.ID,
		Token: "dummy-token", // @todo
	}, nil
}
