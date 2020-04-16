package users

import (
	"context"

	logging "github.com/ipfs/go-log"
	pb "github.com/textileio/textile/api/users/pb"
	c "github.com/textileio/textile/collections"
)

var (
	log = logging.Logger("users")
)

type Service struct {
	Collections *c.Collections
}

func (s *Service) ListThreads(ctx context.Context, _ *pb.ListThreadsRequest) (*pb.ListThreadsReply, error) {
	log.Debugf("received list threads request")

	user, _ := c.UserFromContext(ctx)
	list, err := s.Collections.Threads.List(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	reply := &pb.ListThreadsReply{
		List: make([]*pb.ListThreadsReply_Thread, len(list)),
	}
	for i, t := range list {
		reply.List[i] = &pb.ListThreadsReply_Thread{
			ID:   t.ID.Bytes(),
			Name: t.Name,
		}
	}
	return reply, nil
}
