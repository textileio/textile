package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/util"
	pb "github.com/textileio/textile/v2/api/sendfild/pb"
	"github.com/textileio/textile/v2/api/sendfild/service/interfaces"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	collectionName = "sendfil"
)

var (
	log = logging.Logger("store")
)

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

func (t *txn) LatestMsgCid() (msgCid, error) {
	if len(t.MessageCids) == 0 {
		log.Errorf("no message cid found for txn object id %v", t.ID.Hex())
		return msgCid{}, fmt.Errorf("no message cids found")
	}
	return t.MessageCids[len(t.MessageCids)-1], nil
}

type Store struct {
	col *mongo.Collection
}

func New(mongoUri, mongoDbName string, debug bool) (*Store, error) {
	ctx := context.Background()
	if debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"store": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoUri))
	if err != nil {
		return nil, fmt.Errorf("connecting to mongo: %v", err)
	}
	db := client.Database(mongoDbName)
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
		return nil, fmt.Errorf("creating collection indexes: %v", err)
	}

	s := &Store{
		col: col,
	}

	return s, nil
}

func (s *Store) New(ctx context.Context, messageCid, from, to string, amountNanoFil int64) (*pb.Txn, error) {
	if amountNanoFil <= 0 {
		return nil, fmt.Errorf("amountNanoFil must be greater than 0")
	}

	now := time.Now()

	t := &txn{
		ID:            primitive.NewObjectID(),
		From:          from,
		To:            to,
		AmountNanoFil: amountNanoFil,
		MessageCids:   []msgCid{{Cid: messageCid, CreatedAt: now}},
		MessageState:  pb.MessageState_MESSAGE_STATE_PENDING,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if _, err := s.col.InsertOne(ctx, &t); err != nil {
		return nil, fmt.Errorf("inserting txn into collection: %v", err)
	}

	pbTxn, err := toPbTxn(t)
	if err != nil {
		return nil, fmt.Errorf("converting to pb txn: %v", err)
	}

	return pbTxn, nil
}

func (s *Store) Get(ctx context.Context, messageCid string) (*pb.Txn, error) {
	res := s.col.FindOne(ctx, bson.M{"message_cids.cid": messageCid})
	if res.Err() == mongo.ErrNoDocuments {
		return nil, interfaces.ErrTxnNotFound
	}
	if res.Err() != nil {
		return nil, fmt.Errorf("querying for cid Txn: %v", res.Err())
	}

	var t txn
	if err := res.Decode(&t); err != nil {
		return nil, fmt.Errorf("decoding cid Txn result: %v", err)
	}

	pbTxn, err := toPbTxn(&t)
	if err != nil {
		return nil, fmt.Errorf("converting to pb txn: %v", err)
	}

	return pbTxn, nil
}

func (s *Store) GetAllPending(ctx context.Context, excludeAlreadyWaiting bool) ([]*pb.Txn, error) {
	filter := bson.M{"message_state": pb.MessageState_MESSAGE_STATE_PENDING}
	if excludeAlreadyWaiting {
		filter["waiting"] = false
	}
	cursor, err := s.col.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("querying txns: %v", err)
	}
	defer cursor.Close(ctx)
	var txns []*txn
	if err := cursor.All(ctx, &txns); err != nil {
		return nil, fmt.Errorf("decoding txns query results: %v", err)
	}

	var pbTxns []*pb.Txn
	for _, t := range txns {
		pbTxn, err := toPbTxn(t)
		if err != nil {
			return nil, fmt.Errorf("converting txn to pb: %v", err)
		}
		pbTxns = append(pbTxns, pbTxn)
	}

	return pbTxns, nil
}

func (s *Store) SetWaiting(ctx context.Context, messageCid string, waiting bool) error {
	res := s.col.FindOneAndUpdate(
		ctx,
		bson.M{"message_cids.cid": messageCid},
		bson.M{"$set": bson.M{
			"waiting": waiting,
		}},
	)
	if errors.Is(res.Err(), mongo.ErrNoDocuments) {
		return interfaces.ErrTxnNotFound
	}
	if res.Err() != nil {
		return fmt.Errorf("updating txn: %v", res.Err())
	}
	return nil
}

func (s *Store) Fail(ctx context.Context, messageCid, failureMsg string) error {
	res := s.col.FindOneAndUpdate(
		ctx,
		bson.M{"message_cids.cid": messageCid},
		bson.M{"$set": bson.M{
			"message_state": pb.MessageState_MESSAGE_STATE_FAILED,
			"failure_msg":   failureMsg,
			"waiting":       false,
			"updated_at":    time.Now(),
		}},
	)
	if errors.Is(res.Err(), mongo.ErrNoDocuments) {
		return interfaces.ErrTxnNotFound
	}
	if res.Err() != nil {
		return fmt.Errorf("updating txn: %v", res.Err())
	}
	return nil
}

func (s *Store) Activate(ctx context.Context, knownCid, latestCid string) error {
	res := s.col.FindOne(ctx, bson.M{"message_cids.cid": knownCid})
	if res.Err() == mongo.ErrNoDocuments {
		return interfaces.ErrTxnNotFound
	}
	if res.Err() != nil {
		return fmt.Errorf("querying for cid txn: %v", res.Err())
	}

	var t txn
	if err := res.Decode(&t); err != nil {
		return fmt.Errorf("decoding cid txn result: %v", err)
	}

	now := time.Now()

	t.MessageState = pb.MessageState_MESSAGE_STATE_ACTIVE
	t.Waiting = false
	t.UpdatedAt = now

	// This would probably not ever be true because we would already know about and have tracked
	// the new message cid if we replaced the message with a new one. Checking just in case.
	isNewCid := true
	for _, msgCid := range t.MessageCids {
		if latestCid == msgCid.Cid {
			isNewCid = false
			break
		}
	}

	if isNewCid {
		t.MessageCids = append(t.MessageCids, msgCid{Cid: latestCid, CreatedAt: now})
	}

	updateRes, err := s.col.ReplaceOne(ctx, bson.M{"_id": t.ID}, t)
	if err != nil {
		log.Errorf("calling ReplaceOne to update txn: %v", err)
		return err
	}
	if updateRes.MatchedCount == 0 {
		log.Error("no document matched calling ReplaceOne to update txn")
		return interfaces.ErrTxnNotFound
	}
	return nil
}

func (s *Store) List(ctx context.Context, req *pb.ListTxnsRequest) ([]*pb.Txn, error) {
	findOpts := options.Find()
	findOpts = findOpts.SetLimit(req.PageSize)
	findOpts = findOpts.SetSkip(req.PageSize * req.Page)
	sort := -1
	if req.Ascending {
		sort = 1
	}
	findOpts = findOpts.SetSort(bson.D{primitive.E{Key: "created_at", Value: sort}})

	filter := bson.M{}

	ands := bson.A{}

	// Message cids
	messageCidsOrs := bson.A{}
	for _, messageCid := range req.MessageCidsFilter {
		messageCidsOrs = append(messageCidsOrs, bson.M{"message_cids.cid": messageCid})
	}
	if len(messageCidsOrs) > 0 {
		ands = append(ands, bson.M{"$or": messageCidsOrs})
	}

	// Involving/from/to
	if req.InvolvingAddressFilter != "" {
		ands = append(ands, bson.M{"$or": bson.A{bson.M{"from": req.InvolvingAddressFilter}, bson.M{"to": req.InvolvingAddressFilter}}})
	} else {
		if req.FromFilter != "" {
			ands = append(ands, bson.M{"from": req.FromFilter})
		}
		if req.ToFilter != "" {
			ands = append(ands, bson.M{"to": req.ToFilter})
		}
	}

	// MessageState
	if req.MessageStateFilter != pb.MessageState_MESSAGE_STATE_UNSPECIFIED {
		ands = append(ands, bson.M{"message_state": req.MessageStateFilter})
	}

	// Waiting
	if req.WaitingFilter != pb.WaitingFilter_WAITING_FILTER_UNSPECIFIED {
		ands = append(ands, bson.M{"waiting": req.WaitingFilter == pb.WaitingFilter_WAITING_FILTER_WAITING})
	}

	// Amount eq/gte/lts/gt/lt
	if req.AmountNanoFilEqFilter != 0 {
		ands = append(ands, bson.M{"amount_nano_fil": req.AmountNanoFilEqFilter})
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

	if len(ands) > 0 {
		filter["$and"] = ands
	}

	cursor, err := s.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("querying txns: %v", err)
	}
	defer cursor.Close(ctx)
	var txns []*txn
	err = cursor.All(ctx, &txns)
	if err != nil {
		return nil, fmt.Errorf("decoding txns query results: %v", err)
	}

	var pbTxns []*pb.Txn
	for _, t := range txns {
		pbTxn, err := toPbTxn(t)
		if err != nil {
			return nil, fmt.Errorf("converting txn to pb: %v", err)
		}
		pbTxns = append(pbTxns, pbTxn)
	}

	return pbTxns, nil
}

type EntityCount struct {
	ID    interface{} `bson:"_id"`
	Count int64       `bson:"count"`
}
type Stats struct {
	ID    string  `bson:"_id"`
	Total int64   `bson:"total"`
	Avg   float64 `bson:"avg"`
	Min   int64   `bson:"min"`
	Max   int64   `bson:"max"`
}
type Summary struct {
	All            []EntityCount `bson:"all"`
	ByMessageState []EntityCount `bson:"by_message_state"`
	Waiting        []EntityCount `bson:"waiting"`
	UniqueFrom     []EntityCount `bson:"unique_from"`
	UniqueTo       []EntityCount `bson:"unique_to"`
	SentFilStats   []Stats       `bson:"sent_fil_stats"`
}

func (s *Store) Summary(ctx context.Context, after, before time.Time) (*pb.SummaryResponse, error) {
	createdAtMatch := bson.M{}
	if !before.IsZero() {
		createdAtMatch["$lt"] = before
	}
	if !after.IsZero() {
		createdAtMatch["$gt"] = after
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
		return nil, fmt.Errorf("calling aggregate: %v", err)
	}
	var res []*Summary
	if err = cursor.All(ctx, &res); err != nil {
		return nil, fmt.Errorf("decoding cursor results: %v", err)
	}
	if len(res) != 1 {
		return nil, fmt.Errorf("unexpected number of aggregate results: %v", len(res))
	}

	summary := res[0]

	resp := &pb.SummaryResponse{}

	if len(summary.All) == 1 {
		resp.CountTxns = summary.All[0].Count
	}

	for _, state := range summary.ByMessageState {
		switch pb.MessageState(state.ID.(int32)) {
		case pb.MessageState_MESSAGE_STATE_PENDING:
			resp.CountPending = state.Count
		case pb.MessageState_MESSAGE_STATE_ACTIVE:
			resp.CountActive = state.Count
		case pb.MessageState_MESSAGE_STATE_FAILED:
			resp.CountFailed = state.Count
		}
	}

	if len(summary.Waiting) == 1 {
		resp.CountWaiting = summary.Waiting[0].Count
	}

	resp.CountFromAddrs = int64(len(summary.UniqueFrom))
	resp.CountToAddrs = int64(len(summary.UniqueTo))

	if len(summary.SentFilStats) == 1 {
		resp.TotalNanoFilSent = summary.SentFilStats[0].Total
		resp.AvgNanoFilSent = summary.SentFilStats[0].Avg
		resp.MaxNanoFilSent = summary.SentFilStats[0].Max
		resp.MinNanoFilSent = summary.SentFilStats[0].Min
	}

	return resp, nil
}

func (s *Store) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.col.Database().Client().Disconnect(ctx); err != nil {
		log.Errorf("disconnecting mongo client: %s", err)
		return err
	}

	log.Info("mongo client disconnected")

	return nil
}

func toPbTxn(t *txn) (*pb.Txn, error) {
	latestMsgCid, err := t.LatestMsgCid()
	if err != nil {
		return nil, err
	}
	return &pb.Txn{
		Id:            t.ID.Hex(),
		From:          t.From,
		To:            t.To,
		AmountNanoFil: t.AmountNanoFil,
		MessageCid:    latestMsgCid.Cid,
		MessageState:  t.MessageState,
		Waiting:       t.Waiting,
		FailureMsg:    t.FailureMsg,
		CreatedAt:     timestamppb.New(t.CreatedAt),
		UpdatedAt:     timestamppb.New(t.UpdatedAt),
	}, nil
}
