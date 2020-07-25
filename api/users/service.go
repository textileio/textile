package users

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/crypto"
	coredb "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	pb "github.com/textileio/textile/api/users/pb"
	mdb "github.com/textileio/textile/mongodb"
	tdb "github.com/textileio/textile/threaddb"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	log = logging.Logger("usersapi")

	// ErrInboxNotFound indicates that an inbox has not been setup for a message receiver.
	ErrInboxNotFound = errors.New("message inbox not found")
	// ErrOutboxNotFound indicates that an outbox has not been setup for a message sender.
	ErrOutboxNotFound = errors.New("message outbox not found")
)

const (
	inboxName  = "messages-inbox"
	outboxName = "messages-outbox"
)

type Service struct {
	Collections *mdb.Collections
	Messages    *tdb.Messages
}

func (s *Service) GetThread(ctx context.Context, req *pb.GetThreadRequest) (*pb.GetThreadReply, error) {
	log.Debugf("received get thread request")

	user, ok := mdb.UserFromContext(ctx)
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

	user, ok := mdb.UserFromContext(ctx)
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

func (s *Service) SetupMailboxes(ctx context.Context, _ *pb.SetupMailboxesRequest) (*pb.SetupMailboxesReply, error) {
	log.Debugf("received setup mailboxes request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	inbox, err := s.getOrCreateBox(ctx, user.Key, inboxName, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	outbox, err := s.getOrCreateBox(ctx, user.Key, outboxName, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	return &pb.SetupMailboxesReply{
		InboxID:  inbox.Bytes(),
		OutboxID: outbox.Bytes(),
	}, nil
}

func (s *Service) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageReply, error) {
	log.Debugf("received send message request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	to := &thread.Libp2pPubKey{}
	if err := to.UnmarshalString(req.To); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Invalid public key")
	}
	ok, err := user.Key.Verify(req.Body, req.Signature)
	if !ok || err != nil {
		return nil, status.Error(codes.Unauthenticated, "Bad message signature")
	}
	outbox, err := s.getOutbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	inbox, err := s.getInbox(ctx, to)
	if err != nil {
		return nil, err
	}
	msg := tdb.Message{
		ID:        coredb.NewInstanceID().String(),
		From:      thread.NewLibp2pPubKey(user.Key).String(),
		To:        to.String(),
		Body:      base64.StdEncoding.EncodeToString(req.Body),
		Signature: base64.StdEncoding.EncodeToString(req.Signature),
		CreatedAt: time.Now().UnixNano(),
	}
	if _, err := s.Messages.Create(ctx, outbox, msg, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	if _, err := s.Messages.Create(ctx, inbox, msg, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	return &pb.SendMessageReply{
		ID:        msg.ID,
		CreatedAt: msg.CreatedAt,
	}, nil
}

func (s *Service) ListInboxMessages(ctx context.Context, req *pb.ListInboxMessagesRequest) (*pb.ListMessagesReply, error) {
	log.Debugf("received list inbox messages request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	inbox, err := s.getInbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	list, next, err := s.listMessages(ctx, inbox, req.Seek, req.Limit, int32(req.Status), dbToken)
	if err != nil {
		return nil, err
	}
	return &pb.ListMessagesReply{
		Messages:   list,
		NextOffset: next,
	}, nil
}

func (s *Service) ListOutboxMessages(ctx context.Context, req *pb.ListOutboxMessagesRequest) (*pb.ListMessagesReply, error) {
	log.Debugf("received list outbox messages request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	outbox, err := s.getOutbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	list, next, err := s.listMessages(ctx, outbox, req.Seek, req.Limit, 0, dbToken)
	if err != nil {
		return nil, err
	}
	return &pb.ListMessagesReply{
		Messages:   list,
		NextOffset: next,
	}, nil
}

func (s *Service) listMessages(ctx context.Context, dbID thread.ID, seek string, limit int64, stat int32, token thread.Token) ([]*pb.Message, int64, error) {
	q := (&db.Query{}).LimitTo(int(limit)) //.SkipNum(int(offset))
	//if offset > 0 {
	//	q.And("created_at").Gt(float64(offset))
	//}
	q.Seek = seek
	switch stat {
	case 0:
		break
	case 1:
		q.And("read_at").Gt(float64(0))
	case 2:
		q.And("read_at").Eq(float64(0))
	default:
		return nil, 0, fmt.Errorf("unknown message status")
	}
	res, err := s.Messages.List(ctx, dbID, q, &tdb.Message{}, tdb.WithToken(token))
	if err != nil {
		return nil, 0, err
	}
	list := res.([]*tdb.Message)
	pblist := make([]*pb.Message, len(list))
	var next int64
	for i, m := range list {
		pblist[i], err = messageToPb(m)
		if err != nil {
			return nil, 0, err
		}
		if i == len(list)-1 {
			next = m.CreatedAt
		}
	}
	return pblist, next, nil
}

func messageToPb(m *tdb.Message) (*pb.Message, error) {
	body, err := base64.StdEncoding.DecodeString(m.Body)
	if err != nil {
		return nil, err
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil {
		return nil, err
	}
	return &pb.Message{
		ID:        m.ID,
		From:      m.From,
		To:        m.To,
		Body:      body,
		Signature: sig,
		CreatedAt: m.CreatedAt,
		ReadAt:    m.ReadAt,
	}, nil
}

func (s *Service) ReadInboxMessage(ctx context.Context, req *pb.ReadMessageRequest) (*pb.ReadMessageReply, error) {
	log.Debugf("received read inbox message request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	inbox, err := s.getInbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	msg := &tdb.Message{}
	err = s.Messages.Get(ctx, inbox, req.ID, msg, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	msg.ReadAt = time.Now().UnixNano()
	if err := s.Messages.Save(ctx, inbox, msg, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	return &pb.ReadMessageReply{
		ReadAt: msg.ReadAt,
	}, nil
}

func (s *Service) DeleteInboxMessage(ctx context.Context, req *pb.DeleteMessageRequest) (*pb.DeleteMessageReply, error) {
	log.Debugf("received delete inbox message request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	inbox, err := s.getInbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	if err := s.Messages.Delete(ctx, inbox, req.ID, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	return &pb.DeleteMessageReply{}, nil
}

func (s *Service) DeleteOutboxMessage(ctx context.Context, req *pb.DeleteMessageRequest) (*pb.DeleteMessageReply, error) {
	log.Debugf("received delete outbox message request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	outbox, err := s.getOutbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	if err := s.Messages.Delete(ctx, outbox, req.ID, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	return &pb.DeleteMessageReply{}, nil
}

func (s *Service) getInbox(ctx context.Context, key crypto.PubKey) (thread.ID, error) {
	thrd, err := s.Collections.Threads.GetByName(ctx, inboxName, key)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return thread.Undef, status.Error(codes.FailedPrecondition, ErrInboxNotFound.Error())
		}
		return thread.Undef, err
	}
	return thrd.ID, nil
}

func (s *Service) getOutbox(ctx context.Context, key crypto.PubKey) (thread.ID, error) {
	thrd, err := s.Collections.Threads.GetByName(ctx, outboxName, key)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return thread.Undef, status.Error(codes.FailedPrecondition, ErrOutboxNotFound.Error())
		}
		return thread.Undef, err
	}
	return thrd.ID, nil
}

func (s *Service) getOrCreateBox(ctx context.Context, key crypto.PubKey, name string, opts ...tdb.Option) (thread.ID, error) {
	id, err := s.Messages.NewMailbox(ctx, name, opts...)
	if err != nil && strings.Contains(err.Error(), mdb.DuplicateErrMsg) {
		thrd, err := s.Collections.Threads.GetByName(ctx, name, key)
		if err != nil {
			return thread.Undef, err
		}
		return thrd.ID, nil
	} else if err != nil {
		return thread.Undef, err
	}
	return id, nil
}
