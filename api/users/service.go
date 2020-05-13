package users

import (
	"context"
	"errors"

	logging "github.com/ipfs/go-log"
	pb "github.com/textileio/textile/api/users/pb"
	c "github.com/textileio/textile/collections"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("usersapi")
)

type Service struct {
	Collections *c.Collections
}

func (s *Service) GetThread(ctx context.Context, req *pb.GetThreadRequest) (*pb.GetThreadReply, error) {
	log.Debugf("received get thread request")

	user, ok := c.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	thrd, err := s.Collections.Threads.GetByName(ctx, req.Name, user.Key)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, status.Error(codes.NotFound, "Thread not found")
		}
		return nil, err
	}
	return &pb.GetThreadReply{
		ID:   thrd.ID.Bytes(),
		Name: thrd.Name,
		IsDB: thrd.IsDB,
	}, nil
}

func (s *Service) ListThreads(ctx context.Context, _ *pb.ListThreadsRequest) (*pb.ListThreadsReply, error) {
	log.Debugf("received list threads request")

	user, ok := c.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	list, err := s.Collections.Threads.ListByOwner(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	reply := &pb.ListThreadsReply{
		List: make([]*pb.GetThreadReply, len(list)),
	}
	for i, t := range list {
		reply.List[i] = &pb.GetThreadReply{
			ID:   t.ID.Bytes(),
			Name: t.Name,
			IsDB: t.IsDB,
		}
	}
	return reply, nil
}
