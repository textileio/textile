package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/util"
	pb "github.com/textileio/textile/v2/api/sendfild/pb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	collectionName = "sendfil"
)

var (
	log = logging.Logger("store")

	ErrNotFound = fmt.Errorf("no Txn found")
)

type MsgCid struct {
	Cid       string    `bson:"cid"`
	CreatedAt time.Time `bson:"created_at"`
}

type Txn struct {
	ID            primitive.ObjectID `bson:"_id"`
	From          string             `bson:"from"`
	To            string             `bson:"to"`
	AmountNanoFil int64              `bson:"amount_nano_fil"`
	MessageCids   []MsgCid           `bson:"message_cids"`
	MessageState  pb.MessageState    `bson:"message_state"`
	Waiting       bool               `bson:"waiting"`
	FailureMsg    string             `bson:"failure_msg"`
	CreatedAt     time.Time          `bson:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at"`
}

func (t *Txn) LatestMsgCid() (MsgCid, error) {
	if len(t.MessageCids) == 0 {
		log.Errorf("no message cid found for txn object id %v", t.ID.Hex())
		return MsgCid{}, fmt.Errorf("no message cids found")
	}
	return t.MessageCids[len(t.MessageCids)-1], nil
}

type Store struct {
	col           *mongo.Collection
	mainCtxCancel context.CancelFunc
}

func New(mongoUri, mongoDbName string, debug bool) (*Store, error) {
	ctx, cancel := context.WithCancel(context.Background())
	if debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"store": logging.LevelDebug,
		}); err != nil {
			cancel()
			return nil, err
		}
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoUri))
	if err != nil {
		cancel()
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
		cancel()
		return nil, fmt.Errorf("creating collection indexes: %v", err)
	}

	s := &Store{
		col:           col,
		mainCtxCancel: cancel,
	}

	return s, nil
}

func (s *Store) NewTxn(ctx context.Context, messageCid, from, to string, amountNanoFil int64) (*Txn, error) {
	now := time.Now()

	txn := &Txn{
		ID:            primitive.NewObjectID(),
		From:          from,
		To:            to,
		AmountNanoFil: amountNanoFil,
		MessageCids:   []MsgCid{{Cid: messageCid, CreatedAt: now}},
		MessageState:  pb.MessageState_MESSAGE_STATE_PENDING,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if _, err := s.col.InsertOne(ctx, &txn); err != nil {
		return nil, fmt.Errorf("inserting txn into collection: %v", err)
	}

	return txn, nil
}

func (s *Store) GetTxn(ctx context.Context, messageCid string) (*Txn, error) {
	res := s.col.FindOne(ctx, bson.M{"message_cids.cid": messageCid})
	if res.Err() == mongo.ErrNoDocuments {
		return nil, ErrNotFound
	}
	if res.Err() != nil {
		return nil, fmt.Errorf("querying for cid Txn: %v", res.Err())
	}

	var txn Txn
	if err := res.Decode(&txn); err != nil {
		return nil, fmt.Errorf("decoding cid Txn result: %v", err)
	}

	return &txn, nil
}

func (s *Store) GetAllPending(ctx context.Context, excludeAlreadyWaiting bool) ([]*Txn, error) {
	filter := bson.M{"message_state": pb.MessageState_MESSAGE_STATE_PENDING}
	if excludeAlreadyWaiting {
		filter["waiting"] = false
	}
	cursor, err := s.col.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("querying txns: %v", err)
	}
	defer cursor.Close(ctx)
	var txns []*Txn
	err = cursor.All(ctx, &txns)
	if err != nil {
		return nil, fmt.Errorf("decoding txns query results: %v", err)
	}
	return txns, nil
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
		return ErrNotFound
	}
	if res.Err() != nil {
		return fmt.Errorf("updating txn: %v", res.Err())
	}
	return nil
}

func (s *Store) FailTxn(ctx context.Context, messageCid, failureMsg string) error {
	res := s.col.FindOneAndUpdate(
		ctx,
		bson.M{"message_cids.cid": messageCid},
		bson.M{"$set": bson.M{
			"message_state":   pb.MessageState_MESSAGE_STATE_FAILED,
			"failure_message": failureMsg,
			"waiting":         false,
			"updated_at":      time.Now(),
		}},
	)
	if errors.Is(res.Err(), mongo.ErrNoDocuments) {
		return ErrNotFound
	}
	if res.Err() != nil {
		return fmt.Errorf("updating txn: %v", res.Err())
	}
	return nil
}

func (s *Store) ActivateTxn(ctx context.Context, knownCid, latestCid string) error {
	res := s.col.FindOne(ctx, bson.M{"message_cids.cid": knownCid})
	if res.Err() == mongo.ErrNoDocuments {
		return ErrNotFound
	}
	if res.Err() != nil {
		return fmt.Errorf("querying for cid txn: %v", res.Err())
	}

	var txn Txn
	if err := res.Decode(&txn); err != nil {
		return fmt.Errorf("decoding cid txn result: %v", err)
	}

	now := time.Now()

	txn.MessageState = pb.MessageState_MESSAGE_STATE_ACTIVE
	txn.Waiting = false
	txn.UpdatedAt = now

	// This would probably not ever be true because we would already know about and have tracked
	// the new message cid if we replaced the message with a new one. Checking just in case.
	isNewCid := true
	for _, msgCid := range txn.MessageCids {
		if latestCid == msgCid.Cid {
			isNewCid = false
			break
		}
	}

	if isNewCid {
		txn.MessageCids = append(txn.MessageCids, MsgCid{Cid: latestCid, CreatedAt: now})
	}

	updateRes, err := s.col.ReplaceOne(ctx, bson.M{"_id": txn.ID}, txn)
	if err != nil {
		log.Errorf("calling ReplaceOne to update txn: %v", err)
		return err
	}
	if updateRes.MatchedCount == 0 {
		log.Error("no document matched calling ReplaceOne to update txn")
		return ErrNotFound
	}
	return nil
}

func (s *Store) ListTxns(ctx context.Context, req *pb.ListTxnsRequest) ([]*Txn, error) {
	findOpts := options.Find()
	findOpts = findOpts.SetLimit(req.PageSize)
	findOpts = findOpts.SetSkip(req.PageSize * req.Page)
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

	if len(ands) > 0 {
		filter["$and"] = ands
	}

	cursor, err := s.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("querying txns: %v", err)
	}
	defer cursor.Close(ctx)
	var txns []*Txn
	err = cursor.All(ctx, &txns)
	if err != nil {
		return nil, fmt.Errorf("decoding txns query results: %v", err)
	}

	return txns, nil
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

func (s *Store) GenerateSummary(ctx context.Context, after, before time.Time) (*Summary, error) {
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
	return res[0], nil
}

func (s *Store) Close() error {
	var e error

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.col.Database().Client().Disconnect(ctx); err != nil {
		log.Errorf("disconnecting mongo client: %s", err)
		e = err
	} else {
		log.Info("mongo client disconnected")
	}

	s.mainCtxCancel()

	return e
}
