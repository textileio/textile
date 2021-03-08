package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/chain/types"
	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/powergate/v2/lotus"
	pb "github.com/textileio/textile/v2/api/sendfild/pb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	collectionName = "sendfil"
)

var _ pb.SendFilServiceServer = (*Service)(nil)

var log = logging.Logger("sendfil")

type msgCid struct {
	Cid       string    `bson:"cid"`
	CreatedAt time.Time `bson:"created_at"`
}

type txn struct {
	ID            primitive.ObjectID `bson:"_id"`
	From          string             `bson:"from"`
	To            string             `bson:"to"`
	AmountNanoFil int64              `bson:"amount_nano_fil"`
	MessageCids   []msgCid           `bson:"message_cids"`
	MessageState  pb.MessageState    `bson:"message_state"`
	Waiting       bool               `bson:"waiting"`
	FailureMsg    string             `bson:"failure_msg"`
	CreatedAt     time.Time          `bson:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at"`
}

func (t txn) latestMsgCid() (msgCid, error) {
	if len(t.MessageCids) == 0 {
		log.Errorf("no message cid found for txn object id %v", t.ID.Hex())
		return msgCid{}, fmt.Errorf("no message cids found")
	}
	return t.MessageCids[len(t.MessageCids)-1], nil
}

type Service struct {
	clientBuilder lotus.ClientBuilder
	col           *mongo.Collection
	server        *grpc.Server
	waiting       map[primitive.ObjectID]chan waitResult
	config        Config
	ticker        *time.Ticker
	waitingLck    sync.Mutex
	mainCtxCancel context.CancelFunc
}

type Config struct {
	Listener           net.Listener
	ClientBuilder      lotus.ClientBuilder
	MongoUri           string
	MongoDbName        string
	MessageWaitTimeout time.Duration
	MessageConfidence  uint64
	RetryWaitFrequency time.Duration
	Debug              bool
}

func New(config Config) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())
	if config.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"sendfil": logging.LevelDebug,
		}); err != nil {
			cancel()
			return nil, err
		}
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoUri))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("connecting to mongo: %v", err)
	}
	db := client.Database(config.MongoDbName)
	col := db.Collection(collectionName)
	if _, err := col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{primitive.E{Key: "from", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "to", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "amount_nano_fil", Value: 1}},
		},
		// MongoDB automatically creates a multikey index if any indexed field is an array;
		// you do not need to explicitly specify the multikey type.
		// https://docs.mongodb.com/manual/core/index-multikey/
		{
			Keys:    bson.D{primitive.E{Key: "message_cids.cid", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{primitive.E{Key: "message_state", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "waiting", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "created_at", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "updated_at", Value: 1}},
		},
	}); err != nil {
		cancel()
		return nil, fmt.Errorf("creating collection indexes: %v", err)
	}

	s := &Service{
		clientBuilder: config.ClientBuilder,
		col:           col,
		waiting:       make(map[primitive.ObjectID]chan waitResult),
		config:        config,
		ticker:        time.NewTicker(config.RetryWaitFrequency),
		mainCtxCancel: cancel,
	}

	s.server = grpc.NewServer()
	go func() {
		pb.RegisterSendFilServiceServer(s.server, s)
		if err := s.server.Serve(config.Listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Errorf("serve error: %v", err)
		}
	}()

	if err := s.waitAllPending(ctx, true); err != nil {
		cancel()
		return nil, fmt.Errorf("calling waitAllPending: %v", err)
	}

	s.bindTicker(ctx)

	return s, nil
}

func (s *Service) waitAllPending(ctx context.Context, isInitialRun bool) error {
	filter := bson.M{"message_state": pb.MessageState_MESSAGE_STATE_PENDING}
	if !isInitialRun {
		filter["waiting"] = false
	}
	cursor, err := s.col.Find(ctx, filter)
	if err != nil {
		return status.Errorf(codes.Internal, "querying txns: %v", err)
	}
	defer cursor.Close(ctx)
	var txns []txn
	err = cursor.All(ctx, &txns)
	if err != nil {
		return status.Errorf(codes.Internal, "decoding txns query results: %v", err)
	}
	log.Infof("found %v txns to initiate waiting on", len(txns))
	for _, txn := range txns {
		s.wait(txn)
	}
	return nil
}

func (s *Service) bindTicker(ctx context.Context) {
	go func() {
		for {
			select {
			case <-s.ticker.C:
				if err := s.waitAllPending(ctx, false); err != nil {
					log.Errorf("waitAllPending from ticker: %v", err)
				}
			case <-ctx.Done():
				log.Info("unbinding ticker")
				return
			}
		}
	}()
}

func (s *Service) SendFil(ctx context.Context, req *pb.SendFilRequest) (*pb.SendFilResponse, error) {
	f, err := address.NewFromString(req.From)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "parsing from address: %v", err)
	}
	t, err := address.NewFromString(req.To)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "parsing to address: %v", err)
	}
	nanoAmount := (&big.Int{}).SetInt64(req.AmountNanoFil)
	factor := (&big.Int{}).SetInt64(int64(math.Pow10(9)))
	amount := (&big.Int{}).Mul(nanoAmount, factor)
	msg := &types.Message{
		From:  f,
		To:    t,
		Value: types.BigInt{Int: amount},
	}
	client, cls, err := s.clientBuilder(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating filecoin client: %v", err)
	}
	defer cls()

	sm, err := client.MpoolPushMessage(ctx, msg, nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "pushing message: %v", err)
	}

	now := time.Now()

	tx := txn{
		ID:            primitive.NewObjectID(),
		From:          req.From,
		To:            req.To,
		AmountNanoFil: req.AmountNanoFil,
		MessageCids:   []msgCid{{Cid: sm.Message.Cid().String(), CreatedAt: now}},
		MessageState:  pb.MessageState_MESSAGE_STATE_PENDING,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if _, err = s.col.InsertOne(ctx, &tx); err != nil {
		return nil, status.Errorf(codes.Internal, "inserting txn into collection: %v", err)
	}

	wait := s.wait(tx)

	if req.Wait {
		res := <-wait
		if res.err != nil {
			return nil, status.Errorf(codes.Internal, "waiting for result: %v", res.err)
		}
		tx = res.txn
	}

	pbTx, err := toPbTxn(tx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "converting to pb txn: %v", err)
	}

	return &pb.SendFilResponse{Txn: pbTx}, nil
}

func (s *Service) GetTxn(ctx context.Context, req *pb.GetTxnRequest) (*pb.GetTxnResponse, error) {
	res := s.col.FindOne(ctx, bson.M{"message_cids.cid": req.MessageCid})
	if res.Err() == mongo.ErrNoDocuments {
		return nil, status.Error(codes.NotFound, "no txn found for cid")
	}
	if res.Err() != nil {
		return nil, status.Errorf(codes.Internal, "querying for cid txn: %v", res.Err())
	}

	var tx txn
	if err := res.Decode(&tx); err != nil {
		return nil, status.Errorf(codes.Internal, "decoding cid txn result: %v", res.Err())
	}

	if tx.MessageState == pb.MessageState_MESSAGE_STATE_PENDING && req.Wait {
		res := <-s.wait(tx)
		if res.err != nil {
			return nil, status.Errorf(codes.Internal, "waiting for result: %v", res.err)
		}
		tx = res.txn
	}

	pbTx, err := toPbTxn(tx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "converting to pb txn: %v", err)
	}

	return &pb.GetTxnResponse{Txn: pbTx}, nil
}

func (s *Service) ListTxns(ctx context.Context, req *pb.ListTxnsRequest) (*pb.ListTxnsResponse, error) {
	findOpts := options.Find()
	if req.Limit > 0 {
		findOpts = findOpts.SetLimit(req.Limit)
	}
	sort := -1
	if req.Ascending {
		sort = 1
	}
	findOpts = findOpts.SetSort(bson.D{primitive.E{Key: "created_at", Value: sort}})

	filter := bson.M{}

	// Involving/from/to
	if req.InvolvingAddressFilter != "" {
		filter["$or"] = bson.A{bson.M{"from": req.InvolvingAddressFilter}, bson.M{"to": req.InvolvingAddressFilter}}
	} else {
		if req.FromFilter != "" {
			filter["from"] = req.FromFilter
		}
		if req.ToFilter != "" {
			filter["to"] = req.ToFilter
		}
	}

	// MessageState
	if req.MessageStateFilter != pb.MessageState_MESSAGE_STATE_UNSPECIFIED {
		filter["message_state"] = req.MessageStateFilter
	}

	// Waiting
	if req.WaitingFilter != pb.WaitingFilter_WAITING_FILTER_UNSPECIFIED {
		filter["waiting"] = req.WaitingFilter == pb.WaitingFilter_WAITING_FILTER_WAITING
	}

	ands := bson.A{}

	// Amount eq/gte/lts/gt/lt
	if req.AmountNanoFilEqFilter != 0 {
		filter["amount_nano_fil"] = req.AmountNanoFilEqFilter
	} else {
		if req.AmountNanoFilGteqFilter != 0 {
			ands = append(ands, bson.M{"amount_nano_fil": bson.M{"$gte": req.AmountNanoFilGteqFilter}})
		} else if req.AmountNanoFilGtFilter != 0 {
			ands = append(ands, bson.M{"amount_nano_fil": bson.M{"$gt": req.AmountNanoFilGtFilter}})
		}

		if req.AmountNanoFilLteqFilter != 0 {
			ands = append(ands, bson.M{"amount_nano_fil": bson.M{"$lte": req.AmountNanoFilLteqFilter}})
		} else if req.AmountNanoFilLtFilter != 0 {
			ands = append(ands, bson.M{"amount_nano_fil": bson.M{"$lt": req.AmountNanoFilLtFilter}})
		}
	}

	// Updated after/before
	if req.UpdatedAfter != nil {
		ands = append(ands, bson.M{"updated_at": bson.M{"$gt": req.UpdatedAfter.AsTime()}})
	}
	if req.UpdatedBefore != nil {
		ands = append(ands, bson.M{"updated_at": bson.M{"$lt": req.UpdatedBefore.AsTime()}})
	}

	// Created after/before
	if req.CreatedAfter != nil {
		ands = append(ands, bson.M{"created_at": bson.M{"$gt": req.CreatedAfter.AsTime()}})
	}
	if req.CreatedBefore != nil {
		ands = append(ands, bson.M{"created_at": bson.M{"$lt": req.CreatedBefore.AsTime()}})
	}

	// Apply paging info
	comp := "$lt"
	if req.MoreToken != 0 {
		if req.Ascending {
			comp = "$gt"
		}
		t := time.Unix(0, req.MoreToken)
		ands = append(ands, bson.M{"created_at": bson.M{comp: &t}})
	}

	if len(ands) > 0 {
		filter["$and"] = ands
	}

	cursor, err := s.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "querying txns: %v", err)
	}
	defer cursor.Close(ctx)
	var txns []txn
	err = cursor.All(ctx, &txns)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "decoding txns query results: %v", err)
	}

	more := false
	var startAt *time.Time
	if len(txns) > 0 {
		lastCreatedAt := &txns[len(txns)-1].CreatedAt
		filter["created_at"] = bson.M{comp: *lastCreatedAt}
		res := s.col.FindOne(ctx, filter)
		if res.Err() != nil && res.Err() != mongo.ErrNoDocuments {
			return nil, status.Errorf(codes.Internal, "checking for more data: %v", err)
		}
		if res.Err() != mongo.ErrNoDocuments {
			more = true
			startAt = lastCreatedAt
		}
	}
	var pbTxns []*pb.Txn
	for _, rec := range txns {
		pbTxn, err := toPbTxn(rec)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "converting txn to pb: %v", err)
		}
		pbTxns = append(pbTxns, pbTxn)
	}
	res := &pb.ListTxnsResponse{
		Txns: pbTxns,
		More: more,
	}
	if startAt != nil {
		res.MoreToken = startAt.UnixNano()
	}
	return res, nil
}

func (s *Service) Summary(ctx context.Context, req *pb.SummaryRequest) (*pb.SummaryResponse, error) {
	type entityCount struct {
		ID    interface{} `bson:"_id"`
		Count int64       `bson:"count"`
	}
	type stats struct {
		ID    string  `bson:"_id"`
		Total int64   `bson:"total"`
		Avg   float64 `bson:"avg"`
		Min   int64   `bson:"min"`
		Max   int64   `bson:"max"`
	}
	type report struct {
		All            []entityCount `bson:"all"`
		ByMessageState []entityCount `bson:"by_message_state"`
		Waiting        []entityCount `bson:"waiting"`
		UniqueFrom     []entityCount `bson:"unique_from"`
		UniqueTo       []entityCount `bson:"unique_to"`
		SentFilStats   []stats       `bson:"sent_fil_stats"`
	}

	createdAtMatch := bson.M{}
	if req.Before != nil {
		createdAtMatch["$lt"] = req.Before.AsTime()
	}
	if req.After != nil {
		createdAtMatch["$gt"] = req.After.AsTime()
	}
	match := bson.M{}
	if len(createdAtMatch) > 0 {
		match["created_at"] = createdAtMatch
	}
	cursor, err := s.col.Aggregate(ctx, bson.A{
		bson.M{"$match": match},
		bson.M{"$facet": bson.M{
			"all": bson.A{
				bson.M{"$group": bson.M{"_id": nil, "count": bson.M{"$sum": 1}}},
			},
			"by_message_state": bson.A{
				bson.M{"$group": bson.M{"_id": "$message_state", "count": bson.M{"$sum": 1}}},
			},
			"waiting": bson.A{
				bson.M{"$match": bson.M{"waiting": true}},
				bson.M{"$group": bson.M{"_id": nil, "count": bson.M{"$sum": 1}}},
			},
			"unique_from": bson.A{
				bson.M{"$group": bson.M{"_id": "$from", "count": bson.M{"$sum": 1}}},
			},
			"unique_to": bson.A{
				bson.M{"$group": bson.M{"_id": "$to", "count": bson.M{"$sum": 1}}},
			},
			"sent_fil_stats": bson.A{
				bson.M{"$group": bson.M{
					"_id":   nil,
					"total": bson.M{"$sum": "$amount_nano_fil"},
					"avg":   bson.M{"$avg": "$amount_nano_fil"},
					"max":   bson.M{"$max": "$amount_nano_fil"},
					"min":   bson.M{"$min": "$amount_nano_fil"},
				}},
			},
		}},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "calling aggregate: %v", err)
	}
	var res []report
	if err = cursor.All(ctx, &res); err != nil {
		return nil, status.Errorf(codes.Internal, "decoding cursor results: %v", err)
	}
	if len(res) != 1 {
		return nil, status.Errorf(codes.Internal, "unexpected number of aggregate results: %v", len(res))
	}

	r := res[0]

	resp := &pb.SummaryResponse{}

	if len(r.All) == 1 {
		resp.CountTxns = r.All[0].Count
	}

	for _, state := range r.ByMessageState {
		switch pb.MessageState(state.ID.(int32)) {
		case pb.MessageState_MESSAGE_STATE_PENDING:
			resp.CountPending = state.Count
		case pb.MessageState_MESSAGE_STATE_ACTIVE:
			resp.CountActive = state.Count
		case pb.MessageState_MESSAGE_STATE_FAILED:
			resp.CountFailed = state.Count
		}
	}

	if len(r.Waiting) == 1 {
		resp.CountWaiting = r.Waiting[0].Count
	}

	resp.CountFromAddrs = int64(len(r.UniqueFrom))
	resp.CountToAddrs = int64(len(r.UniqueTo))

	if len(r.SentFilStats) == 1 {
		resp.TotalNanoFilSent = r.SentFilStats[0].Total
		resp.AvgNanoFilSent = r.SentFilStats[0].Avg
		resp.MaxNanoFilSent = r.SentFilStats[0].Max
		resp.MinNanoFilSent = r.SentFilStats[0].Min
	}

	return resp, nil
}

type waitResult struct {
	txn txn
	err error
}

func (s *Service) wait(tx txn) chan waitResult {
	s.waitingLck.Lock()
	defer s.waitingLck.Unlock()

	waitCh, found := s.waiting[tx.ID]
	if found {
		return waitCh
	}

	ch := make(chan waitResult)
	s.waiting[tx.ID] = ch

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), s.config.MessageWaitTimeout)

		client, closeClient, err := s.clientBuilder(ctx)
		defer func() {
			closeClient()
			close(ch)
			cancel()
		}()
		if err != nil {
			log.Errorf("creating lotus client: %v", err)
			ch <- waitResult{err: fmt.Errorf("creating lotus client: %v", err)}
			return
		}

		lastestMsgCid, err := tx.latestMsgCid()
		if err != nil {
			tx.MessageState = pb.MessageState_MESSAGE_STATE_FAILED
			tx.FailureMsg = fmt.Sprintf("getting latest message cid from txn: %v", err)
			tx.UpdatedAt = time.Now()
			if err := s.updateTxn(ctx, tx); err != nil {
				ch <- waitResult{err: err}
				return
			}
			ch <- waitResult{txn: tx}
			return
		}
		c, err := cid.Decode(lastestMsgCid.Cid)
		if err != nil {
			log.Errorf("decoding message cid: %s", tx.FailureMsg)
			tx.MessageState = pb.MessageState_MESSAGE_STATE_FAILED
			tx.FailureMsg = fmt.Sprintf("decoding latest message cid for txn: %v", err)
			tx.UpdatedAt = time.Now()
			if err := s.updateTxn(ctx, tx); err != nil {
				ch <- waitResult{err: err}
				return
			}
			ch <- waitResult{txn: tx}
			return
		}

		tx.Waiting = true
		if err := s.updateTxn(ctx, tx); err != nil {
			ch <- waitResult{err: err}
			return
		}

		res, err := client.StateWaitMsg(ctx, c, s.config.MessageConfidence)
		tx.Waiting = false
		if err != nil {
			// If for some reason the lotus node doesn't know about the cid, consider that a final error.
			if strings.Contains(err.Error(), "block not found") {
				tx.MessageState = pb.MessageState_MESSAGE_STATE_FAILED
				tx.FailureMsg = err.Error()
				tx.UpdatedAt = time.Now()
				log.Warnf("failing txn with err: %s", tx.FailureMsg)
			}
			if err := s.updateTxn(ctx, tx); err != nil {
				ch <- waitResult{err: err}
				return
			}
			if tx.MessageState == pb.MessageState_MESSAGE_STATE_FAILED {
				ch <- waitResult{txn: tx}
			} else {
				ch <- waitResult{err: fmt.Errorf("calling StateWaitMsg: %v", err)}
			}
			return
		}

		if res.Receipt.ExitCode.IsError() {
			tx.MessageState = pb.MessageState_MESSAGE_STATE_FAILED
			tx.FailureMsg = fmt.Sprintf("error exit code: %v", res.Receipt.ExitCode.Error())
			tx.UpdatedAt = time.Now()
			log.Warnf("failing txn with err: %s", tx.FailureMsg)
			if err := s.updateTxn(ctx, tx); err != nil {
				ch <- waitResult{err: err}
				return
			}
			ch <- waitResult{txn: tx}
			if res.Receipt.ExitCode.IsSendFailure() {
				log.Errorf("received exit code send failure: %s", res.Receipt.ExitCode.String())
			} else {
				log.Infof("received exit code error: %s", res.Receipt.ExitCode.String())
			}
			return
		}

		tx.MessageState = pb.MessageState_MESSAGE_STATE_ACTIVE
		tx.UpdatedAt = time.Now()

		// This would probably not ever be true because we would already know about and have tracked
		// the new message cid if we replaced the message with a new one. Checking just in case.
		isNewCid := true
		for _, msgCid := range tx.MessageCids {
			if res.Message.String() == msgCid.Cid {
				isNewCid = false
				break
			}
		}

		if isNewCid {
			tx.MessageCids = append(tx.MessageCids, msgCid{Cid: res.Message.String(), CreatedAt: time.Now()})
		}

		if err := s.updateTxn(ctx, tx); err != nil {
			ch <- waitResult{err: err}
			return
		}

		s.waitingLck.Lock()
		delete(s.waiting, tx.ID)
		s.waitingLck.Unlock()

		ch <- waitResult{txn: tx}
	}()

	return ch
}

func (s *Service) Close() error {
	var e error

	s.ticker.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.col.Database().Client().Disconnect(ctx); err != nil {
		log.Errorf("disconnecting mongo client: %s", err)
		e = err
	} else {
		log.Info("mongo client disconnected")
	}

	stopped := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(stopped)
	}()
	t := time.NewTimer(10 * time.Second)
	select {
	case <-t.C:
		s.server.Stop()
	case <-stopped:
		t.Stop()
	}
	log.Info("gRPC server stopped")

	s.mainCtxCancel()

	return e
}

func (s *Service) updateTxn(ctx context.Context, t txn) error {
	res, err := s.col.ReplaceOne(ctx, bson.M{"_id": t.ID}, &t)
	if err != nil {
		log.Errorf("calling ReplaceOne to update txn: %v", err)
		return err
	}
	if res.MatchedCount == 0 {
		log.Error("no document matched calling ReplaceOne to update txn")
		return fmt.Errorf("no matched txn document to replace")
	}
	return nil
}

func toPbTxn(txn txn) (*pb.Txn, error) {
	latestMsgCid, err := txn.latestMsgCid()
	if err != nil {
		return nil, err
	}
	return &pb.Txn{
		Id:            txn.ID.Hex(),
		From:          txn.From,
		To:            txn.To,
		AmountNanoFil: txn.AmountNanoFil,
		MessageCid:    latestMsgCid.Cid,
		MessageState:  txn.MessageState,
		Waiting:       txn.Waiting,
		FailureMsg:    txn.FailureMsg,
		CreatedAt:     timestamppb.New(txn.CreatedAt),
		UpdatedAt:     timestamppb.New(txn.UpdatedAt),
	}, nil
}
