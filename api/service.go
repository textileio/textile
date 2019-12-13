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

// SignUp handles a signup request.
func (s *service) SignUp(ctx context.Context, req *pb.SignUpRequest) (*pb.SignUpReply, error) {
	log.Debugf("received sign up request")

	user := &users.User{}
	if err := s.users.Create(user); err != nil {
		return nil, err
	}

	return &pb.SignUpReply{ID: user.ID}, nil
}
