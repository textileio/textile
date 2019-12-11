package api

import (
	"context"

	"github.com/textileio/go-textile-threads/api/client"
	pb "github.com/textileio/textile/api/pb"
)

// service is a gRPC service for textile.
type service struct {
	threads *client.Client
}

// NewUser adds a new user to the user store.
func (s *service) NewUser(ctx context.Context, req *pb.NewUserRequest) (*pb.NewUserReply, error) {
	log.Debugf("received new user request")

	return &pb.NewUserReply{}, nil
}
