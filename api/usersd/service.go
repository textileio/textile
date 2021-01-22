package usersd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	ulid "github.com/oklog/ulid/v2"
	coredb "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/go-threads/db"
	pow "github.com/textileio/powergate/v2/api/client"
	userPb "github.com/textileio/powergate/v2/api/gen/powergate/user/v1"
	billing "github.com/textileio/textile/v2/api/billingd/client"
	pb "github.com/textileio/textile/v2/api/usersd/pb"
	"github.com/textileio/textile/v2/buckets/archive/retrieval"
	"github.com/textileio/textile/v2/mail"
	mdb "github.com/textileio/textile/v2/mongodb"
	tdb "github.com/textileio/textile/v2/threaddb"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var log = logging.Logger("usersapi")

type Service struct {
	Collections     *mdb.Collections
	Mail            *tdb.Mail
	BillingClient   *billing.Client
	FilRetrieval    *retrieval.FilRetrieval
	PowergateClient *pow.Client
}

func (s *Service) GetThread(ctx context.Context, req *pb.GetThreadRequest) (*pb.GetThreadResponse, error) {
	log.Debugf("received get thread request")

	account, _ := mdb.AccountFromContext(ctx)
	thrd, err := s.Collections.Threads.GetByName(ctx, req.Name, account.Owner().Key)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, status.Error(codes.NotFound, "Thread not found")
		}
		return nil, err
	}
	return &pb.GetThreadResponse{
		Id:   thrd.ID.Bytes(),
		Name: thrd.Name,
		IsDb: thrd.IsDB,
	}, nil
}

func (s *Service) ListThreads(ctx context.Context, _ *pb.ListThreadsRequest) (*pb.ListThreadsResponse, error) {
	log.Debugf("received list threads request")

	account, _ := mdb.AccountFromContext(ctx)
	list, err := s.Collections.Threads.ListByOwner(ctx, account.Owner().Key)
	if err != nil {
		return nil, err
	}
	reply := &pb.ListThreadsResponse{
		List: make([]*pb.GetThreadResponse, len(list)),
	}
	for i, t := range list {
		reply.List[i] = &pb.GetThreadResponse{
			Id:   t.ID.Bytes(),
			Name: t.Name,
			IsDb: t.IsDB,
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

func (s *Service) SetupMailbox(ctx context.Context, _ *pb.SetupMailboxRequest) (*pb.SetupMailboxResponse, error) {
	log.Debugf("received setup mailbox request")

	account, _ := mdb.AccountFromContext(ctx)
	dbToken, _ := thread.TokenFromContext(ctx)

	box, err := s.getOrCreateMailbox(ctx, account.Owner().Key, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	return &pb.SetupMailboxResponse{
		MailboxId: box.Bytes(),
	}, nil
}

func (s *Service) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	log.Debugf("received send message request")

	account, _ := mdb.AccountFromContext(ctx)
	dbToken, _ := thread.TokenFromContext(ctx)

	ok, err := account.Owner().Key.Verify(req.ToBody, req.ToSignature)
	if !ok || err != nil {
		return nil, status.Error(codes.Unauthenticated, "Bad message signature")
	}
	ok, err = account.Owner().Key.Verify(req.FromBody, req.FromSignature)
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
	sentbox, err := s.getMailbox(ctx, account.Owner().Key)
	if err != nil {
		return nil, err
	}

	msgID := coredb.NewInstanceID().String()
	now := time.Now().UnixNano()
	from := account.Owner().Key
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
	return &pb.SendMessageResponse{
		Id:        msgID,
		CreatedAt: now,
	}, nil
}

func (s *Service) ListInboxMessages(ctx context.Context, req *pb.ListInboxMessagesRequest) (*pb.ListInboxMessagesResponse, error) {
	log.Debugf("received list inbox messages request")

	account, _ := mdb.AccountFromContext(ctx)
	dbToken, _ := thread.TokenFromContext(ctx)

	box, err := s.getMailbox(ctx, account.Owner().Key)
	if err != nil {
		return nil, err
	}
	query, err := getMailboxQuery(req.Seek, req.Limit, req.Ascending, req.Status)
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
	return &pb.ListInboxMessagesResponse{Messages: pblist}, nil
}

func (s *Service) ListSentboxMessages(ctx context.Context, req *pb.ListSentboxMessagesRequest) (*pb.ListSentboxMessagesResponse, error) {
	log.Debugf("received list sentbox messages request")

	account, _ := mdb.AccountFromContext(ctx)
	dbToken, _ := thread.TokenFromContext(ctx)

	box, err := s.getMailbox(ctx, account.Owner().Key)
	if err != nil {
		return nil, err
	}
	query, err := getMailboxQuery(req.Seek, req.Limit, req.Ascending, pb.ListInboxMessagesRequest_STATUS_ALL)
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
	return &pb.ListSentboxMessagesResponse{Messages: pblist}, nil
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
		Id:        m.ID,
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
		Id:        m.ID,
		From:      m.From,
		To:        m.To,
		Body:      body,
		Signature: sig,
		CreatedAt: m.CreatedAt,
	}, nil
}

func getMailboxQuery(seek string, limit int64, asc bool, stat pb.ListInboxMessagesRequest_Status) (q *db.Query, err error) {
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
	case pb.ListInboxMessagesRequest_STATUS_ALL:
	case pb.ListInboxMessagesRequest_STATUS_UNSPECIFIED:
		break
	case pb.ListInboxMessagesRequest_STATUS_READ:
		q.And("read_at").Gt(minMessageReadAt)
	case pb.ListInboxMessagesRequest_STATUS_UNREAD:
		q.And("read_at").Eq(minMessageReadAt)
	default:
		return nil, fmt.Errorf("unknown message status: %v", stat.String())
	}
	return q, nil
}

func (s *Service) ReadInboxMessage(ctx context.Context, req *pb.ReadInboxMessageRequest) (*pb.ReadInboxMessageResponse, error) {
	log.Debugf("received read inbox message request")

	account, _ := mdb.AccountFromContext(ctx)
	dbToken, _ := thread.TokenFromContext(ctx)

	box, err := s.getMailbox(ctx, account.Owner().Key)
	if err != nil {
		return nil, err
	}
	msg := &tdb.InboxMessage{}
	err = s.Mail.Inbox.Get(ctx, box, req.Id, msg, tdb.WithToken(dbToken))
	if err != nil {
		return nil, err
	}
	msg.ReadAt = time.Now().UnixNano()
	if err := s.Mail.Inbox.Save(ctx, box, msg, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	return &pb.ReadInboxMessageResponse{
		ReadAt: msg.ReadAt,
	}, nil
}

func (s *Service) DeleteInboxMessage(ctx context.Context, req *pb.DeleteInboxMessageRequest) (*pb.DeleteInboxMessageResponse, error) {
	log.Debugf("received delete inbox message request")

	account, _ := mdb.AccountFromContext(ctx)
	dbToken, _ := thread.TokenFromContext(ctx)

	box, err := s.getMailbox(ctx, account.Owner().Key)
	if err != nil {
		return nil, err
	}
	if err := s.Mail.Inbox.Delete(ctx, box, req.Id, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	return &pb.DeleteInboxMessageResponse{}, nil
}

func (s *Service) DeleteSentboxMessage(ctx context.Context, req *pb.DeleteSentboxMessageRequest) (*pb.DeleteSentboxMessageResponse, error) {
	log.Debugf("received delete sentbox message request")

	account, _ := mdb.AccountFromContext(ctx)
	dbToken, _ := thread.TokenFromContext(ctx)

	box, err := s.getMailbox(ctx, account.Owner().Key)
	if err != nil {
		return nil, err
	}
	if err := s.Mail.Sentbox.Delete(ctx, box, req.Id, tdb.WithToken(dbToken)); err != nil {
		return nil, err
	}
	return &pb.DeleteSentboxMessageResponse{}, nil
}

func (s *Service) GetUsage(ctx context.Context, req *pb.GetUsageRequest) (*pb.GetUsageResponse, error) {
	log.Debugf("received get usage request")

	if s.BillingClient == nil {
		return nil, fmt.Errorf("billing is not enabled")
	}

	account, _ := mdb.AccountFromContext(ctx)
	var key, parentKey thread.PubKey
	if req.Key != "" {
		k := &thread.Libp2pPubKey{}
		if err := k.UnmarshalString(req.Key); err != nil {
			return nil, status.Error(codes.FailedPrecondition, "Invalid public key")
		}
		key = k
		parentKey = account.Owner().Key
	} else {
		key = account.Owner().Key
	}
	cus, err := s.BillingClient.GetCustomer(ctx, key)
	if err != nil {
		return nil, err
	}
	if parentKey != nil && cus.ParentKey != parentKey.String() {
		return nil, status.Error(codes.NotFound, "User not found")
	}
	usage, err := s.BillingClient.GetCustomerUsage(ctx, key)
	if err != nil {
		return nil, err
	}
	return &pb.GetUsageResponse{
		Customer: cus,
		Usage:    usage,
	}, nil
}

func (s *Service) getMailbox(ctx context.Context, key thread.PubKey) (thread.ID, error) {
	thrd, err := s.Collections.Threads.GetByName(ctx, mail.ThreadName, key)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return thread.Undef, status.Error(codes.FailedPrecondition, ErrMailboxNotFound.Error())
		}
		return thread.Undef, err
	}
	return thrd.ID, nil
}

func (s *Service) getOrCreateMailbox(ctx context.Context, key thread.PubKey, opts ...tdb.Option) (thread.ID, error) {
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

// ArchiveRetrievalLs lists existing retrievals for the account.
func (s *Service) ArchiveRetrievalLs(ctx context.Context, req *pb.ArchiveRetrievalLsRequest) (*pb.ArchiveRetrievalLsResponse, error) {
	account, _ := mdb.AccountFromContext(ctx)
	owner := account.Owner().Key

	rs, err := s.FilRetrieval.GetAllByAccount(owner.String())
	if err != nil {
		return nil, fmt.Errorf("listing retrievals: %s", err)
	}

	res := &pb.ArchiveRetrievalLsResponse{
		Retrievals: make([]*pb.ArchiveRetrievalLsItem, len(rs)),
	}
	for i, r := range rs {
		res.Retrievals[i] = &pb.ArchiveRetrievalLsItem{
			Id:           r.JobID,
			Cid:          r.Cid.String(),
			Status:       toPbRetrievalStatus(r.Status),
			FailureCause: r.FailureCause,
			CreatedAt:    r.CreatedAt,
		}
		switch r.Type {
		case retrieval.TypeNewBucket:
			rt := &pb.ArchiveRetrievalLsItem_NewBucket{
				NewBucket: &pb.ArchiveRetrievalLsItemNewBucket{
					Name:    r.Name,
					Private: r.Private,
				},
			}
			res.Retrievals[i].RetrievalType = rt
		default:
			return nil, fmt.Errorf("unkown retrieval type")
		}
	}

	return res, nil
}

// ArchivesLs lists all known archives for an account.
func (s *Service) ArchivesLs(ctx context.Context, req *pb.ArchivesLsRequest) (*pb.ArchivesLsResponse, error) {
	account, _ := mdb.AccountFromContext(ctx)
	if account.Owner().PowInfo == nil {
		return nil, fmt.Errorf("no powergate info associated with account")
	}

	ctx = context.WithValue(ctx, pow.AuthKey, account.Owner().PowInfo.Token)
	r, err := s.PowergateClient.Data.CidSummary(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting archived cids: %s", err)
	}

	res := &pb.ArchivesLsResponse{
		Archives: make([]*pb.ArchiveLsItem, len(r.CidSummary)),
	}
	for i, cs := range r.CidSummary {
		ci, err := s.PowergateClient.Data.CidInfo(ctx, cs.Cid)
		if err != nil {
			return nil, fmt.Errorf("getting cid info: %s", err)
		}
		props := ci.CidInfo.CurrentStorageInfo.Cold.Filecoin.Proposals
		ali := &pb.ArchiveLsItem{
			Cid:  cs.Cid,
			Info: make([]*pb.ArchiveLsItemMetadata, len(props)),
		}
		res.Archives[i] = ali

		for j, p := range props {
			ali.Info[j] = &pb.ArchiveLsItemMetadata{
				DealId: uint64(p.DealId),
			}
		}
	}

	return res, nil
}

// ArchivesImport imports Filecoin deals information for a Cid to the account.
func (s *Service) ArchivesImport(ctx context.Context, req *pb.ArchivesImportRequest) (*pb.ArchivesImportResponse, error) {
	account, _ := mdb.AccountFromContext(ctx)
	if account.Owner().PowInfo == nil {
		return nil, fmt.Errorf("no powergate info associated with account")
	}

	var scfg *userPb.StorageConfig

	ctx = context.WithValue(ctx, pow.AuthKey, account.Owner().PowInfo.Token)
	ci, err := s.PowergateClient.Data.CidInfo(ctx, req.Cid)
	var notFound bool
	if err != nil {
		sc, ok := status.FromError(err)
		if !ok || sc.Code() != codes.NotFound {
			return nil, fmt.Errorf("getting current storage information: %s", err)
		}
		notFound = true
	}

	// If deal import is for a new Cid, just use the default Storage Config
	// with both storages disabled and without running any jobs: only deal importing.
	if notFound {
		defConfRes, err := s.PowergateClient.StorageConfig.Default(ctx)
		if err != nil {
			return nil, fmt.Errorf("getting default storage-config: %s", err)
		}
		scfg = defConfRes.DefaultStorageConfig
		scfg.Cold.Enabled = false
		scfg.Hot.Enabled = false

	} else {
		// If deal import is to augment an existing Cid, just use the latest storage config.
		// A Job won't run anyway, so it would only import the deals.
		scfg = ci.CidInfo.LatestPushedStorageConfig
	}

	if _, err = s.PowergateClient.StorageConfig.Apply(
		ctx,
		req.Cid,
		pow.WithStorageConfig(scfg),
		pow.WithOverride(true),
		pow.WithImportDealIDs(req.DealIds),
		pow.WithNoExec(true),
	); err != nil {
		return nil, fmt.Errorf("importing deals: %s", err)
	}

	return &pb.ArchivesImportResponse{}, nil
}

// ArchiveRetrievalLogs prints the logs of a retrieval.
func (s *Service) ArchiveRetrievalLogs(req *pb.ArchiveRetrievalLogsRequest, server pb.APIService_ArchiveRetrievalLogsServer) error {
	account, _ := mdb.AccountFromContext(server.Context())
	owner := account.Owner()
	accKey := owner.Key.String()
	powToken := owner.PowInfo.Token

	var err error
	ctx, cancel := context.WithCancel(server.Context())
	defer cancel()
	ch := make(chan string)
	go func() {
		err = s.FilRetrieval.Logs(ctx, accKey, req.Id, powToken, ch)
		close(ch)
	}()
	for s := range ch {
		if err := server.Send(&pb.ArchiveRetrievalLogsResponse{Msg: s}); err != nil {
			return err
		}
	}
	if err != nil {
		return fmt.Errorf("watching retrieval logs: %s", err)
	}

	return nil
}

func toPbRetrievalStatus(s retrieval.Status) pb.ArchiveRetrievalStatus {
	switch s {
	case retrieval.StatusQueued:
		return pb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_QUEUED
	case retrieval.StatusExecuting:
		return pb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_EXECUTING
	case retrieval.StatusMoveToBucket:
		return pb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_MOVETOBUCKET
	case retrieval.StatusSuccess:
		return pb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_SUCCESS
	case retrieval.StatusFailed:
		return pb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_FAILED
	default:
		return pb.ArchiveRetrievalStatus_ARCHIVE_RETRIEVAL_STATUS_UNSPECIFIED
	}
}
