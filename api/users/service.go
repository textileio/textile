package users

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/oklog/ulid"
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

	// ErrInboxNotFound indicates that an inbox has not been setup for a mail receiver.
	ErrInboxNotFound = errors.New("inbox not found")
	// ErrSentboxNotFound indicates that a sent mailbox has not been setup for a mail sender.
	ErrSentboxNotFound = errors.New("sentbox not found")
)

const (
	inboxName   = "messages-inbox"
	sentboxName = "messages-sent"
)

type Service struct {
	Collections *mdb.Collections
	Mail        *tdb.Mail
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

func (s *Service) SetupMail(ctx context.Context, _ *pb.SetupMailRequest) (*pb.SetupMailReply, error) {
	log.Debugf("received setup mail request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	inbox, err := s.getOrCreateMailbox(ctx, user.Key, inboxName, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	sentbox, err := s.getOrCreateMailbox(ctx, user.Key, sentboxName, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	return &pb.SetupMailReply{
		InboxID: inbox.Bytes(),
		SentID:  sentbox.Bytes(),
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
	sentbox, err := s.getSentbox(ctx, user.Key)
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
	if _, err := s.Mail.Create(ctx, sentbox, msg, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	if _, err := s.Mail.Create(ctx, inbox, msg, tdb.WithToken(dbToken)); err != nil {
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
	list, err := s.listMessages(ctx, inbox, req.Seek, req.Limit, int32(req.Status), dbToken)
	if err != nil {
		return nil, err
	}
	return &pb.ListMessagesReply{Messages: list}, nil
}

func (s *Service) ListSentMessages(ctx context.Context, req *pb.ListSentMessagesRequest) (*pb.ListMessagesReply, error) {
	log.Debugf("received list sent messages request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	sentbox, err := s.getSentbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	list, err := s.listMessages(ctx, sentbox, req.Seek, req.Limit, 0, dbToken)
	if err != nil {
		return nil, err
	}
	return &pb.ListMessagesReply{Messages: list}, nil
}

func (s *Service) listMessages(ctx context.Context, dbID thread.ID, seek string, limit int64, stat int32, token thread.Token) ([]*pb.Message, error) {
	if seek == "" {
		seek = ulid.MustNew(ulid.MaxTime(), rand.Reader).String()
	}
	q := db.OrderByIDDesc().SeekID(coredb.InstanceID(seek)).LimitTo(int(limit))
	switch stat {
	case 0:
		break
	case 1:
		q.And("read_at").Gt(float64(0))
	case 2:
		q.And("read_at").Eq(float64(0))
	default:
		return nil, fmt.Errorf("unknown message status")
	}
	res, err := s.Mail.List(ctx, dbID, q, &tdb.Message{}, tdb.WithToken(token))
	if err != nil {
		return nil, err
	}
	list := res.([]*tdb.Message)
	pblist := make([]*pb.Message, len(list))
	for i, m := range list {
		pblist[i], err = messageToPb(m)
		if err != nil {
			return nil, err
		}
	}
	return pblist, nil
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
	err = s.Mail.Get(ctx, inbox, req.ID, msg, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	msg.ReadAt = time.Now().UnixNano()
	if err := s.Mail.Save(ctx, inbox, msg, tdb.WithToken(dbToken)); err != nil {
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
	if err := s.Mail.Delete(ctx, inbox, req.ID, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	return &pb.DeleteMessageReply{}, nil
}

func (s *Service) DeleteSentMessage(ctx context.Context, req *pb.DeleteMessageRequest) (*pb.DeleteMessageReply, error) {
	log.Debugf("received delete sent message request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	sentbox, err := s.getSentbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	if err := s.Mail.Delete(ctx, sentbox, req.ID, tdb.WithToken(dbToken)); err != nil {
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

func (s *Service) getSentbox(ctx context.Context, key crypto.PubKey) (thread.ID, error) {
	thrd, err := s.Collections.Threads.GetByName(ctx, sentboxName, key)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return thread.Undef, status.Error(codes.FailedPrecondition, ErrSentboxNotFound.Error())
		}
		return thread.Undef, err
	}
	return thrd.ID, nil
}

func (s *Service) getOrCreateMailbox(ctx context.Context, key crypto.PubKey, name string, opts ...tdb.Option) (thread.ID, error) {
	id, err := s.Mail.NewMailbox(ctx, name, opts...)
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
