package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/util"
	analytics "github.com/textileio/textile/v2/api/analyticsd/client"
	analyticspb "github.com/textileio/textile/v2/api/analyticsd/pb"
	pb "github.com/textileio/textile/v2/api/filrewardsd/pb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var log = logging.Logger("filrewards")

var rewardMeta = map[pb.Reward]meta{
	pb.Reward_REWARD_FIRST_KEY_ACCOUNT_CREATED:    {factor: 3},
	pb.Reward_REWARD_FIRST_KEY_USER_CREATED:       {factor: 1},
	pb.Reward_REWARD_FIRST_ORG_CREATED:            {factor: 3},
	pb.Reward_REWARD_INITIAL_BILLING_SETUP:        {factor: 1},
	pb.Reward_REWARD_FIRST_BUCKET_CREATED:         {factor: 2},
	pb.Reward_REWARD_FIRST_BUCKET_ARCHIVE_CREATED: {factor: 2},
	pb.Reward_REWARD_FIRST_MAILBOX_CREATED:        {factor: 1},
	pb.Reward_REWARD_FIRST_THREAD_DB_CREATED:      {factor: 1},
}

type meta struct {
	factor int32
}

type rewardRecord struct {
	Key               string                  `bson:"key"`
	AccountType       analyticspb.AccountType `bson:"account_type"`
	Reward            pb.Reward               `bson:"reward"`
	Factor            int32                   `bson:"factor"`
	BaseAttoFILReward int32                   `bson:"base_atto_fil_reward"`
	CreatedAt         time.Time               `bson:"created_at"`
	ClaimedAt         *time.Time              `bson:"claimed_at"`
}

var _ pb.FilRewardsServiceServer = (*Service)(nil)

type Service struct {
	col               *mongo.Collection
	ac                *analytics.Client
	rewardsCache      map[string]map[pb.Reward]struct{}
	baseAttoFILReward int32
	server            *grpc.Server
}

type Config struct {
	Listener            net.Listener
	MongoUri            string
	MongoDbName         string
	MongoCollectionName string
	AnalyticsAddr       string
	BaseAttoFILReward   int32
	Debug               bool
}

func New(ctx context.Context, config Config) (*Service, error) {
	if config.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"filrewards": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoUri))
	if err != nil {
		return nil, fmt.Errorf("connecting to mongo: %v", err)
	}
	db := client.Database(config.MongoDbName)
	col := db.Collection(config.MongoCollectionName)
	_, err = col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{primitive.E{Key: "key", Value: 1}, primitive.E{Key: "reward", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		{
			Keys: bson.D{primitive.E{Key: "key", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "reward", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "created_at", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "claimed_at", Value: 1}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating indexes: %v", err)
	}

	// Populate cache.
	opts := options.Find()
	cursor, err := col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("querying RewardRecords to populate cache: %s", err)
	}
	defer cursor.Close(ctx)
	cache := make(map[string]map[pb.Reward]struct{})
	for cursor.Next(ctx) {
		var rec rewardRecord
		if err := cursor.Decode(&rec); err != nil {
			return nil, fmt.Errorf("decoding RewardRecord while building cache: %v", err)
		}
		ensureKeyRewardCache(cache, rec.Key)
		cache[rec.Key][rec.Reward] = struct{}{}
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterating cursor while building cache: %v", err)
	}

	s := &Service{
		col:               col,
		rewardsCache:      cache,
		baseAttoFILReward: config.BaseAttoFILReward,
	}

	if config.AnalyticsAddr != "" {
		s.ac, err = analytics.New(config.AnalyticsAddr)
		if err != nil {
			return nil, fmt.Errorf("creating analytics client: %s", err)
		}
	}

	s.server = grpc.NewServer()
	go func() {
		pb.RegisterFilRewardsServiceServer(s.server, s)
		if err := s.server.Serve(config.Listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Errorf("serve error: %v", err)
		}
	}()

	return s, nil
}

func (s *Service) ProcessAnalyticsEvent(ctx context.Context, req *pb.ProcessAnalyticsEventRequest) (*pb.ProcessAnalyticsEventResponse, error) {
	reward := pb.Reward_REWARD_UNSPECIFIED
	switch req.AnalyticsEvent {
	case analyticspb.Event_EVENT_KEY_ACCOUNT_CREATED:
		reward = pb.Reward_REWARD_FIRST_KEY_ACCOUNT_CREATED
	case analyticspb.Event_EVENT_KEY_USER_CREATED:
		reward = pb.Reward_REWARD_FIRST_KEY_USER_CREATED
	case analyticspb.Event_EVENT_ORG_CREATED:
		reward = pb.Reward_REWARD_FIRST_ORG_CREATED
	case analyticspb.Event_EVENT_BILLING_SETUP:
		reward = pb.Reward_REWARD_INITIAL_BILLING_SETUP
	case analyticspb.Event_EVENT_BUCKET_CREATED:
		reward = pb.Reward_REWARD_FIRST_BUCKET_CREATED
	case analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED:
		reward = pb.Reward_REWARD_FIRST_BUCKET_ARCHIVE_CREATED
	case analyticspb.Event_EVENT_MAILBOX_CREATED:
		reward = pb.Reward_REWARD_FIRST_MAILBOX_CREATED
	case analyticspb.Event_EVENT_THREAD_DB_CREATED:
		reward = pb.Reward_REWARD_FIRST_THREAD_DB_CREATED
	}
	if reward == pb.Reward_REWARD_UNSPECIFIED {
		// It is normal to get an analytics event we aren't interested in, so just return an empty result and no error.
		return &pb.ProcessAnalyticsEventResponse{}, nil
	}

	ensureKeyRewardCache(s.rewardsCache, req.Key)

	if _, exists := s.rewardsCache[req.Key][reward]; exists {
		// This reward is already granted so bail.
		return &pb.ProcessAnalyticsEventResponse{}, nil
	}

	rec := &rewardRecord{
		Key:               req.Key,
		AccountType:       req.AccountType,
		Reward:            reward,
		Factor:            rewardMeta[reward].factor,
		BaseAttoFILReward: s.baseAttoFILReward,
		CreatedAt:         time.Now(),
	}

	if _, err := s.col.InsertOne(ctx, rec); err != nil {
		return nil, status.Errorf(codes.Internal, "inserting reward record: %v", err)
	}

	s.rewardsCache[req.Key][reward] = struct{}{}

	if err := s.ac.Track(
		ctx,
		rec.Key,
		rec.AccountType,
		analyticspb.Event_EVENT_FIL_REWARD_RECORDED,
		analytics.WithProperties(map[string]interface{}{
			"reward":               rec.Reward,
			"factor":               rewardMeta[rec.Reward].factor,
			"base_atto_fil_reward": s.baseAttoFILReward,
		}),
	); err != nil {
		log.Errorf("calling analytics track: %v", err)
	}

	return &pb.ProcessAnalyticsEventResponse{RewardRecord: toPbRewardRecord(rec)}, nil
}

func (s *Service) Claim(ctx context.Context, req *pb.ClaimRequest) (*pb.ClaimResponse, error) {
	rec, err := s.get(ctx, req.Key, req.Reward)
	if err == mongo.ErrNoDocuments {
		return nil, status.Error(codes.NotFound, "reward record not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting reward record: %v", err)
	}
	if rec.ClaimedAt != nil {
		return nil, status.Error(codes.AlreadyExists, "reward already claimed")
	}
	filter := bson.M{"key": req.Key, "reward": req.Reward}
	now := time.Now()
	update := bson.M{"$set": bson.M{"claimed_at": &now}}
	res, err := s.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "updating reward record: %v", err)
	}
	if res.ModifiedCount == 0 {
		return nil, status.Error(codes.Internal, "modified 0 documents trying to update reward record")
	}
	return &pb.ClaimResponse{}, nil
}

func (s *Service) List(ctx context.Context, req *pb.ListRequest) (*pb.ListResponse, error) {
	findOpts := options.Find()
	if req.Limit > 0 {
		findOpts.Limit = &req.Limit
	}
	sort := -1
	if req.Ascending {
		sort = 1
	}
	findOpts.Sort = bson.D{primitive.E{Key: "created_at", Value: sort}}
	filter := bson.M{}
	if req.KeyFilter != "" {
		filter["key"] = req.KeyFilter
	}
	if req.RewardFilter != pb.Reward_REWARD_UNSPECIFIED {
		filter["reward"] = req.RewardFilter
	}
	if req.ClaimedFilter == pb.ClaimedFilter_CLAIMED_FILTER_CLAIMED {
		filter["claimed_at"] = bson.M{"$ne": nil}
	}
	if req.ClaimedFilter == pb.ClaimedFilter_CLAIMED_FILTER_UNCLAIMED {
		filter["claimed_at"] = bson.M{"$eq": nil}
	}
	comp := "$lt"
	if req.StartAt != nil {
		if req.Ascending {
			comp = "$gt"
		}
		t := req.StartAt.AsTime()
		filter["created_at"] = bson.M{comp: &t}
	}
	cursor, err := s.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "querying reward records: %v", err)
	}
	defer cursor.Close(ctx)
	var recs []rewardRecord
	err = cursor.All(ctx, &recs)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "decoding reward record query results: %v", err)
	}

	more := false
	var startAt *time.Time
	if len(recs) > 0 {
		lastCreatedAt := &recs[len(recs)-1].CreatedAt
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
	var pbRecs []*pb.RewardRecord
	for _, rec := range recs {
		pbRecs = append(pbRecs, toPbRewardRecord(&rec))
	}
	res := &pb.ListResponse{
		RewardRecords: pbRecs,
		More:          more,
	}
	if startAt != nil {
		res.MoreStartAt = timestamppb.New(*startAt)
	}
	return res, nil
}

func (s *Service) Get(ctx context.Context, req *pb.GetRequest) (*pb.GetResponse, error) {
	rec, err := s.get(ctx, req.Key, req.Reward)
	if err == mongo.ErrNoDocuments {
		return nil, status.Error(codes.NotFound, "reward record not found")
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting reward record: %v", err)
	}
	return &pb.GetResponse{
		RewardRecord: toPbRewardRecord(rec),
	}, nil
}

func (s *Service) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.col.Database().Client().Disconnect(ctx); err != nil {
		log.Errorf("disconnecting mongo client: %s", err)
	}
	log.Info("mongo client disconnected")

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
}

func (s *Service) get(ctx context.Context, key string, reward pb.Reward) (*rewardRecord, error) {
	filter := bson.M{"key": key, "reward": reward}
	res := s.col.FindOne(ctx, filter)
	if res.Err() != nil {
		return nil, res.Err()
	}
	var rec rewardRecord
	if err := res.Decode(&rec); err != nil {
		return nil, err
	}
	return &rec, nil
}

func toPbRewardRecord(rec *rewardRecord) *pb.RewardRecord {
	res := &pb.RewardRecord{
		Key:               rec.Key,
		AccountType:       rec.AccountType,
		Reward:            rec.Reward,
		Factor:            rec.Factor,
		BaseAttoFilReward: rec.BaseAttoFILReward,
		CreatedAt:         timestamppb.New(rec.CreatedAt),
	}
	if rec.ClaimedAt != nil {
		res.ClaimedAt = timestamppb.New(*rec.ClaimedAt)
	}
	return res
}

func ensureKeyRewardCache(keyCache map[string]map[pb.Reward]struct{}, key string) {
	if _, exists := keyCache[key]; !exists {
		keyCache[key] = map[pb.Reward]struct{}{}
	}
}
