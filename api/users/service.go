package users

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log"
	"github.com/libp2p/go-libp2p-core/crypto"
	ulid "github.com/oklog/ulid/v2"
	coredb "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	pb "github.com/textileio/textile/api/users/pb"
	"github.com/textileio/textile/mail"
	mdb "github.com/textileio/textile/mongodb"
	tdb "github.com/textileio/textile/threaddb"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var log = logging.Logger("usersapi")

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

const (
	defaultMessagePageSize = 100
	maxMessagePageSize     = 10000
	minMessageReadAt       = float64(0)
)

var (
	// ErrMailboxNotFound indicates that a mailbox has not been setup for a mail sender/receiver.
	ErrMailboxNotFound = errors.New("mail not found")
)

func (s *Service) SetupMailbox(ctx context.Context, _ *pb.SetupMailboxRequest) (*pb.SetupMailboxReply, error) {
	log.Debugf("received setup mailbox request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	box, err := s.getOrCreateMailbox(ctx, user.Key, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	return &pb.SetupMailboxReply{
		MailboxID: box.Bytes(),
	}, nil
}

func (s *Service) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageReply, error) {
	log.Debugf("received send message request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	ok, err := user.Key.Verify(req.ToBody, req.ToSignature)
	if !ok || err != nil {
		return nil, status.Error(codes.Unauthenticated, "Bad message signature")
	}
	ok, err = user.Key.Verify(req.FromBody, req.FromSignature)
	if !ok || err != nil {
		return nil, status.Error(codes.Unauthenticated, "Bad message signature")
	}

	to := &thread.Libp2pPubKey{}
	if err := to.UnmarshalString(req.To); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "Invalid public key")
	}
	inbox, err := s.getMailbox(ctx, to)
	if err != nil {
		return nil, err
	}
	sentbox, err := s.getMailbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}

	msgID := coredb.NewInstanceID().String()
	now := time.Now().UnixNano()
	from := thread.NewLibp2pPubKey(user.Key)
	toMsg := tdb.InboxMessage{
		ID:        msgID,
		From:      from.String(),
		To:        to.String(),
		Body:      base64.StdEncoding.EncodeToString(req.ToBody),
		Signature: base64.StdEncoding.EncodeToString(req.ToSignature),
		CreatedAt: now,
	}
	if _, err := s.Mail.Inbox.Create(ctx, inbox, toMsg, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	fromMsg := tdb.SentboxMessage{
		ID:        msgID,
		From:      from.String(),
		To:        to.String(),
		Body:      base64.StdEncoding.EncodeToString(req.FromBody),
		Signature: base64.StdEncoding.EncodeToString(req.FromSignature),
		CreatedAt: now,
	}
	if _, err := s.Mail.Sentbox.Create(ctx, sentbox, fromMsg, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	return &pb.SendMessageReply{
		ID:        msgID,
		CreatedAt: now,
	}, nil
}

func (s *Service) ListInboxMessages(ctx context.Context, req *pb.ListInboxMessagesRequest) (*pb.ListMessagesReply, error) {
	log.Debugf("received list inbox messages request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	box, err := s.getMailbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	query, err := getMailboxQuery(req.Seek, req.Limit, req.Ascending, int32(req.Status))
	if err != nil {
		return nil, err
	}
	res, err := s.Mail.Inbox.List(ctx, box, query, &tdb.InboxMessage{}, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	list := res.([]*tdb.InboxMessage)
	pblist := make([]*pb.Message, len(list))
	for i, m := range list {
		pblist[i], err = inboxMessageToPb(m)
		if err != nil {
			return nil, err
		}
	}
	return &pb.ListMessagesReply{Messages: pblist}, nil
}

func (s *Service) ListSentboxMessages(ctx context.Context, req *pb.ListSentboxMessagesRequest) (*pb.ListMessagesReply, error) {
	log.Debugf("received list sentbox messages request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	box, err := s.getMailbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	query, err := getMailboxQuery(req.Seek, req.Limit, req.Ascending, 0)
	if err != nil {
		return nil, err
	}
	res, err := s.Mail.Sentbox.List(ctx, box, query, &tdb.SentboxMessage{}, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	list := res.([]*tdb.SentboxMessage)
	pblist := make([]*pb.Message, len(list))
	for i, m := range list {
		pblist[i], err = sentboxMessageToPb(m)
		if err != nil {
			return nil, err
		}
	}
	return &pb.ListMessagesReply{Messages: pblist}, nil
}

func inboxMessageToPb(m *tdb.InboxMessage) (*pb.Message, error) {
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

func sentboxMessageToPb(m *tdb.SentboxMessage) (*pb.Message, error) {
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
	}, nil
}

func getMailboxQuery(seek string, limit int64, asc bool, stat int32) (q *db.Query, err error) {
	if asc {
		q = db.OrderByID()
		if seek != "" {
			q.SeekID(coredb.InstanceID(seek))
		}
	} else {
		q = db.OrderByIDDesc()
		if seek == "" {
			seek = ulid.MustNew(ulid.MaxTime(), rand.Reader).String()
		}
		q.SeekID(coredb.InstanceID(seek))
	}
	if limit == 0 {
		limit = defaultMessagePageSize
	} else if limit > maxMessagePageSize {
		limit = maxMessagePageSize
	}
	q.LimitTo(int(limit))
	switch stat {
	case 0:
		break
	case 1:
		q.And("read_at").Gt(minMessageReadAt)
	case 2:
		q.And("read_at").Eq(minMessageReadAt)
	default:
		return nil, fmt.Errorf("unknown message status")
	}
	return q, nil
}

func (s *Service) ReadInboxMessage(ctx context.Context, req *pb.ReadInboxMessageRequest) (*pb.ReadInboxMessageReply, error) {
	log.Debugf("received read inbox message request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	box, err := s.getMailbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	msg := &tdb.InboxMessage{}
	err = s.Mail.Inbox.Get(ctx, box, req.ID, msg, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	msg.ReadAt = time.Now().UnixNano()
	if err := s.Mail.Inbox.Save(ctx, box, msg, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	return &pb.ReadInboxMessageReply{
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

	box, err := s.getMailbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	if err := s.Mail.Inbox.Delete(ctx, box, req.ID, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	return &pb.DeleteMessageReply{}, nil
}

func (s *Service) DeleteSentboxMessage(ctx context.Context, req *pb.DeleteMessageRequest) (*pb.DeleteMessageReply, error) {
	log.Debugf("received delete sentbox message request")

	user, ok := mdb.UserFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	dbToken, _ := thread.TokenFromContext(ctx)

	box, err := s.getMailbox(ctx, user.Key)
	if err != nil {
		return nil, err
	}
	if err := s.Mail.Sentbox.Delete(ctx, box, req.ID, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	return &pb.DeleteMessageReply{}, nil
}

func (s *Service) getMailbox(ctx context.Context, key crypto.PubKey) (thread.ID, error) {
	thrd, err := s.Collections.Threads.GetByName(ctx, mail.ThreadName, key)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return thread.Undef, status.Error(codes.FailedPrecondition, ErrMailboxNotFound.Error())
		}
		return thread.Undef, err
	}
	return thrd.ID, nil
}

func (s *Service) getOrCreateMailbox(ctx context.Context, key crypto.PubKey, opts ...tdb.Option) (thread.ID, error) {
	id, err := s.Mail.NewMailbox(ctx, opts...)
	if errors.Is(err, tdb.ErrMailboxExists) {
		thrd, err := s.Collections.Threads.GetByName(ctx, mail.ThreadName, key)
		if err != nil {
			return thread.Undef, err
		}
		return thrd.ID, nil
	} else if err != nil {
		return thread.Undef, err
	}
	return id, nil
}
