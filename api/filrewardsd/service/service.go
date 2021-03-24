package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"time"

	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/core/thread"
	nutil "github.com/textileio/go-threads/net/util"
	"github.com/textileio/go-threads/util"
	pow "github.com/textileio/powergate/v2/api/client"
	analytics "github.com/textileio/textile/v2/api/analyticsd/client"
	analyticspb "github.com/textileio/textile/v2/api/analyticsd/pb"
	pb "github.com/textileio/textile/v2/api/filrewardsd/pb"
	sendfil "github.com/textileio/textile/v2/api/sendfild/client"
	sendfilpb "github.com/textileio/textile/v2/api/sendfild/pb"
	mdb "github.com/textileio/textile/v2/mongodb"
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
	rewardsCollectionName = "filrewards"
	claimsCollectionName  = "filclaims"
	listMaxPageSize       = 100
)

var log = logging.Logger("filrewards")

var rewardTypeMeta = map[pb.RewardType]meta{
	pb.RewardType_REWARD_TYPE_FIRST_KEY_ACCOUNT_CREATED:    {factor: 3},
	pb.RewardType_REWARD_TYPE_FIRST_KEY_USER_CREATED:       {factor: 1},
	pb.RewardType_REWARD_TYPE_FIRST_ORG_CREATED:            {factor: 3},
	pb.RewardType_REWARD_TYPE_INITIAL_BILLING_SETUP:        {factor: 1},
	pb.RewardType_REWARD_TYPE_FIRST_BUCKET_CREATED:         {factor: 2},
	pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED: {factor: 2},
	pb.RewardType_REWARD_TYPE_FIRST_MAILBOX_CREATED:        {factor: 1},
	pb.RewardType_REWARD_TYPE_FIRST_THREAD_DB_CREATED:      {factor: 1},
}

type meta struct {
	factor int64
}

type reward struct {
	OrgKey            string        `bson:"org_key"`
	DevKey            string        `bson:"dev_key"`
	Type              pb.RewardType `bson:"type"`
	Factor            int64         `bson:"factor"`
	BaseNanoFILReward int64         `bson:"base_nano_fil_reward"`
	CreatedAt         time.Time     `bson:"created_at"`
}

type claim struct {
	ID            primitive.ObjectID `bson:"_id"`
	OrgKey        string             `bson:"org_key"`
	ClaimedBy     string             `bson:"claimed_by"`
	AmountNanoFil int64              `bson:"amount_nano_fil"`
	TxnCid        string             `bson:"txn_cid"`
	CreatedAt     time.Time          `bson:"created_at"`
}

type orgKeyLock string

func (l orgKeyLock) Key() string {
	return string(l)
}

var _ nutil.SemaphoreKey = (*orgKeyLock)(nil)

var _ pb.FilRewardsServiceServer = (*Service)(nil)

type Service struct {
	config            Config
	rewardsCol        *mongo.Collection
	claimsCol         *mongo.Collection
	accounts          *mdb.Accounts
	ac                *analytics.Client
	sc                *sendfil.Client
	pc                *pow.Client
	rewardsCacheOrg   map[string]map[pb.RewardType]struct{}
	rewardsCacheDev   map[string]map[pb.RewardType]struct{}
	baseNanoFILReward int64
	server            *grpc.Server
	semaphores        *nutil.SemaphorePool
	mainCtxCancel     context.CancelFunc
}

type Config struct {
	Listener              net.Listener
	MongoUri              string
	MongoFilRewardsDbName string
	MongoAccountsDbName   string
	AnalyticsAddr         string
	SendfilClientConn     *grpc.ClientConn
	PowAddr               string
	BaseNanoFILReward     int64
	SendFromAddr          string
	IsDevnet              bool
	Debug                 bool
}

func New(config Config) (*Service, error) {
	if config.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"filrewards": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.MongoUri))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("connecting to mongo: %v", err)
	}
	db := client.Database(config.MongoFilRewardsDbName)
	rewardsCol := db.Collection(rewardsCollectionName)
	claimsCol := db.Collection(claimsCollectionName)
	if _, err := rewardsCol.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{primitive.E{Key: "org_key", Value: 1}, primitive.E{Key: "type", Value: 1}},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"org_key": bson.M{"$gt": ""}}),
		},
		{
			Keys:    bson.D{primitive.E{Key: "dev_key", Value: 1}, primitive.E{Key: "type", Value: 1}},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"dev_key": bson.M{"$gt": ""}}),
		},
		{
			Keys: bson.D{primitive.E{Key: "org_key", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "dev_key", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "type", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "created_at", Value: 1}},
		},
	}); err != nil {
		cancel()
		return nil, fmt.Errorf("creating rewards collection indexes: %v", err)
	}

	if _, err := claimsCol.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{primitive.E{Key: "org_key", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "claimed_by", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "created_at", Value: 1}},
		},
	}); err != nil {
		cancel()
		return nil, fmt.Errorf("creating claims collection indexes: %v", err)
	}

	// Populate caches.
	cursor, err := rewardsCol.Find(ctx, bson.M{}, options.Find())
	if err != nil {
		cancel()
		return nil, fmt.Errorf("querying RewardRecords to populate cache: %s", err)
	}
	defer cursor.Close(ctx)
	cacheOrg := make(map[string]map[pb.RewardType]struct{})
	cacheDev := make(map[string]map[pb.RewardType]struct{})
	for cursor.Next(ctx) {
		var rec reward
		if err := cursor.Decode(&rec); err != nil {
			cancel()
			return nil, fmt.Errorf("decoding RewardRecord while building cache: %v", err)
		}
		ensureKeyRewardCache(cacheOrg, rec.OrgKey)
		ensureKeyRewardCache(cacheDev, rec.DevKey)
		cacheOrg[rec.OrgKey][rec.Type] = struct{}{}
		cacheDev[rec.DevKey][rec.Type] = struct{}{}
	}
	if err := cursor.Err(); err != nil {
		cancel()
		return nil, fmt.Errorf("iterating cursor while building cache: %v", err)
	}

	s := &Service{
		config:            config,
		rewardsCol:        rewardsCol,
		claimsCol:         claimsCol,
		rewardsCacheOrg:   cacheOrg,
		rewardsCacheDev:   cacheDev,
		baseNanoFILReward: config.BaseNanoFILReward,
		semaphores:        nutil.NewSemaphorePool(1),
		mainCtxCancel:     cancel,
	}

	if config.AnalyticsAddr != "" {
		s.ac, err = analytics.New(config.AnalyticsAddr, grpc.WithInsecure())
		if err != nil {
			cancel()
			return nil, fmt.Errorf("creating analytics client: %s", err)
		}
	}

	s.accounts, err = mdb.NewAccounts(ctx, client.Database(config.MongoAccountsDbName))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating accounts client: %s", err)
	}

	s.sc, err = sendfil.New(config.SendfilClientConn)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating sendfil client: %s", err)
	}

	s.pc, err = pow.NewClient(config.PowAddr, grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating pow client: %s", err)
	}

	if config.IsDevnet {
		res, err := s.pc.Admin.Wallet.Addresses(ctx)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("getting addrs from powergate: %v", err)
		}
		if len(res.Addresses) == 0 {
			cancel()
			return nil, fmt.Errorf("no addrs returned from powergate")
		}
		s.config.SendFromAddr = res.Addresses[0]
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
	if req.OrgKey == "" {
		return nil, status.Error(codes.InvalidArgument, "must provide org key")
	}
	if req.DevKey == "" {
		return nil, status.Error(codes.InvalidArgument, "must provide dev key")
	}
	if req.AnalyticsEvent == analyticspb.Event_EVENT_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "must provide analytics event")
	}

	lck := s.semaphores.Get(orgKeyLock(req.OrgKey))
	lck.Acquire()
	defer lck.Release()

	t := pb.RewardType_REWARD_TYPE_UNSPECIFIED
	switch req.AnalyticsEvent {
	case analyticspb.Event_EVENT_KEY_ACCOUNT_CREATED:
		t = pb.RewardType_REWARD_TYPE_FIRST_KEY_ACCOUNT_CREATED
	case analyticspb.Event_EVENT_KEY_USER_CREATED:
		t = pb.RewardType_REWARD_TYPE_FIRST_KEY_USER_CREATED
	case analyticspb.Event_EVENT_ORG_CREATED:
		t = pb.RewardType_REWARD_TYPE_FIRST_ORG_CREATED
	case analyticspb.Event_EVENT_BILLING_SETUP:
		t = pb.RewardType_REWARD_TYPE_INITIAL_BILLING_SETUP
	case analyticspb.Event_EVENT_BUCKET_CREATED:
		t = pb.RewardType_REWARD_TYPE_FIRST_BUCKET_CREATED
	case analyticspb.Event_EVENT_BUCKET_ARCHIVE_CREATED:
		t = pb.RewardType_REWARD_TYPE_FIRST_BUCKET_ARCHIVE_CREATED
	case analyticspb.Event_EVENT_MAILBOX_CREATED:
		t = pb.RewardType_REWARD_TYPE_FIRST_MAILBOX_CREATED
	case analyticspb.Event_EVENT_THREAD_DB_CREATED:
		t = pb.RewardType_REWARD_TYPE_FIRST_THREAD_DB_CREATED
	}
	if t == pb.RewardType_REWARD_TYPE_UNSPECIFIED {
		// It is normal to get an analytics event we aren't interested in, so just return an empty result and no error.
		return &pb.ProcessAnalyticsEventResponse{}, nil
	}

	ensureKeyRewardCache(s.rewardsCacheOrg, req.OrgKey)
	ensureKeyRewardCache(s.rewardsCacheDev, req.DevKey)

	if _, exists := s.rewardsCacheOrg[req.OrgKey][t]; exists {
		// This reward is already granted to the Org so bail.
		return &pb.ProcessAnalyticsEventResponse{}, nil
	}

	if _, exists := s.rewardsCacheDev[req.DevKey][t]; exists {
		// This reward is already granted to the Dev so bail.
		return &pb.ProcessAnalyticsEventResponse{}, nil
	}

	r := &reward{
		OrgKey:            req.OrgKey,
		DevKey:            req.DevKey,
		Type:              t,
		Factor:            rewardTypeMeta[t].factor,
		BaseNanoFILReward: s.baseNanoFILReward,
		CreatedAt:         time.Now(),
	}

	if _, err := s.rewardsCol.InsertOne(ctx, r); err != nil {
		return nil, status.Errorf(codes.Internal, "inserting reward: %v", err)
	}

	s.rewardsCacheOrg[req.OrgKey][t] = struct{}{}
	s.rewardsCacheDev[req.DevKey][t] = struct{}{}

	if err := s.ac.Track(
		ctx,
		r.OrgKey,
		analyticspb.AccountType_ACCOUNT_TYPE_ORG,
		analyticspb.Event_EVENT_FIL_REWARD,
		analytics.WithProperties(map[string]interface{}{
			"type":                 pb.RewardType_name[int32(r.Type)],
			"factor":               rewardTypeMeta[r.Type].factor,
			"base_nano_fil_reward": s.baseNanoFILReward,
			"amount_nano_fil":      rewardTypeMeta[r.Type].factor * s.baseNanoFILReward,
			"amount_fil":           float64(rewardTypeMeta[r.Type].factor*s.baseNanoFILReward) / math.Pow10(9),
			"dev_key":              req.DevKey,
		}),
	); err != nil {
		log.Errorf("calling analytics track: %v", err)
	}

	return &pb.ProcessAnalyticsEventResponse{Reward: toPbReward(r)}, nil
}

func (s *Service) ListRewards(ctx context.Context, req *pb.ListRewardsRequest) (*pb.ListRewardsResponse, error) {
	if req.PageSize > listMaxPageSize || req.PageSize == 0 {
		req.PageSize = listMaxPageSize
	}

	findOpts := options.Find()
	findOpts = findOpts.SetLimit(req.PageSize)
	findOpts = findOpts.SetSkip(req.PageSize * req.Page)
	sort := -1
	if req.Ascending {
		sort = 1
	}
	findOpts = findOpts.SetSort(bson.D{primitive.E{Key: "created_at", Value: sort}})
	filter := bson.M{}
	if req.OrgKeyFilter != "" {
		filter["org_key"] = req.OrgKeyFilter
	}
	if req.DevKeyFilter != "" {
		filter["dev_key"] = req.DevKeyFilter
	}
	if req.RewardTypeFilter != pb.RewardType_REWARD_TYPE_UNSPECIFIED {
		filter["type"] = req.RewardTypeFilter
	}

	cursor, err := s.rewardsCol.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "querying rewards: %v", err)
	}
	defer cursor.Close(ctx)
	var rewards []reward
	err = cursor.All(ctx, &rewards)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "decoding reward query results: %v", err)
	}

	var pbRewards []*pb.Reward
	for _, rec := range rewards {
		pbRewards = append(pbRewards, toPbReward(&rec))
	}
	res := &pb.ListRewardsResponse{
		Rewards: pbRewards,
	}
	return res, nil
}

func (s *Service) Claim(ctx context.Context, req *pb.ClaimRequest) (*pb.ClaimResponse, error) {
	lck := s.semaphores.Get(orgKeyLock(req.OrgKey))
	lck.Acquire()
	defer lck.Release()

	totalNanoFilRewarded, err := s.totalNanoFilRewarded(ctx, req.OrgKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "calculating total rewarded: %v", err)
	}
	totalNanoFilClaimedPending, totalNanoFilClaimedComplete, err := s.totalNanoFilClaimed(ctx, req.OrgKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "calculating total claimed: %v", err)
	}

	availableNanoFil := totalNanoFilRewarded - totalNanoFilClaimedPending - totalNanoFilClaimedComplete

	if req.AmountNanoFil > availableNanoFil {
		return nil, status.Errorf(codes.InvalidArgument, "claim amount %d nano fil is greater than available reward balance %d nano fil", req.AmountNanoFil, availableNanoFil)
	}

	pubKey := &thread.Libp2pPubKey{}
	if err := pubKey.UnmarshalString(req.OrgKey); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "unmarshaling org key: %v", err)
	}

	org, err := s.accounts.Get(ctx, pubKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting account: %v", err)
	}
	if org.Type != mdb.Org {
		return nil, status.Error(codes.InvalidArgument, "provided account key is not an org")
	}

	ctxPow := context.WithValue(ctx, pow.AuthKey, org.PowInfo.Token)
	addrsRes, err := s.pc.Wallet.Addresses(ctxPow)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting org addresses: %v", err)
	}

	var toAddr string
	if len(addrsRes.Addresses) == 0 {
		return nil, status.Error(codes.Internal, "no filecoin addresses found")
	} else if len(addrsRes.Addresses) > 1 && req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "multiple addresses found for org, you must provide an address")
	} else if len(addrsRes.Addresses) > 1 && req.Address != "" {
		found := false
		for _, addrInfo := range addrsRes.Addresses {
			if addrInfo.Address == req.Address {
				found = true
				break
			}
		}
		if !found {
			return nil, status.Error(codes.InvalidArgument, "specified address not found in org")
		}
		toAddr = req.Address
	} else {
		toAddr = addrsRes.Addresses[0].Address
	}

	txn, err := s.sc.SendFil(ctx, s.config.SendFromAddr, toAddr, req.AmountNanoFil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sending fil: %v", err)
	}

	t := time.Now()
	c := &claim{
		ID:            primitive.NewObjectID(),
		OrgKey:        req.OrgKey,
		ClaimedBy:     req.ClaimedBy,
		AmountNanoFil: req.AmountNanoFil,
		TxnCid:        txn.MessageCid,
		CreatedAt:     t,
	}

	if _, err := s.claimsCol.InsertOne(ctx, c); err != nil {
		return nil, status.Errorf(codes.Internal, "inserting claim: %v", err)
	}

	if err := s.ac.Track(
		ctx,
		c.OrgKey,
		analyticspb.AccountType_ACCOUNT_TYPE_ORG,
		analyticspb.Event_EVENT_FIL_CLAIM,
		analytics.WithProperties(map[string]interface{}{
			"id":              c.ID.Hex(),
			"claimed_by":      c.ClaimedBy,
			"amount_nano_fil": c.AmountNanoFil,
			"amount_fil":      float64(c.AmountNanoFil) / math.Pow10(9),
			"txn_cid":         txn.MessageCid,
		}),
	); err != nil {
		log.Errorf("calling analytics track: %v", err)
	}

	return &pb.ClaimResponse{Claim: toPbClaim(c, txn)}, nil
}

func (s *Service) ListClaims(ctx context.Context, req *pb.ListClaimsRequest) (*pb.ListClaimsResponse, error) {
	if req.PageSize > listMaxPageSize || req.PageSize == 0 {
		req.PageSize = listMaxPageSize
	}

	findOpts := options.Find()
	findOpts = findOpts.SetLimit(req.PageSize)
	findOpts = findOpts.SetSkip(req.PageSize * req.Page)
	sort := -1
	if req.Ascending {
		sort = 1
	}
	findOpts = findOpts.SetSort(bson.D{primitive.E{Key: "created_at", Value: sort}})
	filter := bson.M{}
	if req.OrgKeyFilter != "" {
		filter["org_key"] = req.OrgKeyFilter
	}
	if req.ClaimedByFilter != "" {
		filter["claimed_by"] = req.ClaimedByFilter
	}

	cursor, err := s.claimsCol.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "querying claims: %v", err)
	}
	defer cursor.Close(ctx)
	var claims []claim
	err = cursor.All(ctx, &claims)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "decoding claim query results: %v", err)
	}

	if len(claims) == 0 {
		return &pb.ListClaimsResponse{}, nil
	}

	var cids []string
	for _, claim := range claims {
		cids = append(cids, claim.TxnCid)
	}

	txns, err := s.sc.ListTxns(ctx, sendfil.ListTxnsMessageCids(cids))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting txns: %v", err)
	}

	txnsLookup := make(map[string]*sendfilpb.Txn)
	for _, txn := range txns {
		txnsLookup[txn.MessageCid] = txn
	}

	var pbClaims []*pb.Claim
	for _, claim := range claims {
		txn, ok := txnsLookup[claim.TxnCid]
		if !ok {
			return nil, status.Errorf(codes.Internal, "no txn data for cid %s", claim.TxnCid)
		}
		pbClaims = append(pbClaims, toPbClaim(&claim, txn))
	}
	res := &pb.ListClaimsResponse{
		Claims: pbClaims,
	}
	return res, nil
}

func (s *Service) Balance(ctx context.Context, req *pb.BalanceRequest) (*pb.BalanceResponse, error) {
	lck := s.semaphores.Get(orgKeyLock(req.OrgKey))
	lck.Acquire()
	defer lck.Release()

	totalNanoFilRewarded, err := s.totalNanoFilRewarded(ctx, req.OrgKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "calculating total rewarded: %v", err)
	}
	totalNanoFilClaimedPending, totalNanoFilClaimedComplete, err := s.totalNanoFilClaimed(ctx, req.OrgKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "calculating total claimed: %v", err)
	}
	return &pb.BalanceResponse{
		RewardedNanoFil:        totalNanoFilRewarded,
		ClaimedPendingNanoFil:  totalNanoFilClaimedPending,
		ClaimedCompleteNanoFil: totalNanoFilClaimedComplete,
		AvailableNanoFil:       totalNanoFilRewarded - totalNanoFilClaimedPending - totalNanoFilClaimedComplete,
	}, nil
}

func (s *Service) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var e error
	if err := s.rewardsCol.Database().Client().Disconnect(ctx); err != nil {
		e = err
		log.Errorf("disconnecting mongo client: %s", err)
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

func (s *Service) totalNanoFilRewarded(ctx context.Context, orgKey string) (int64, error) {
	cursor, err := s.rewardsCol.Aggregate(ctx, bson.A{
		bson.M{"$match": bson.M{"org_key": orgKey}},
		bson.M{"$project": bson.M{"amt": bson.M{"$multiply": bson.A{"$factor", "$base_nano_fil_reward"}}}},
		bson.M{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$amt"}}},
	})
	if err != nil {
		return 0, err
	}
	for cursor.Next(ctx) {
		elements, err := cursor.Current.Elements()
		if err != nil {
			return 0, err
		}
		for _, e := range elements {
			if e.Key() == "total" {
				return e.Value().Int64(), nil
			}
		}
	}
	return 0, nil
}

func (s *Service) totalNanoFilClaimed(ctx context.Context, orgKey string) (int64, int64, error) {
	findOpts := options.Find().SetProjection(bson.M{"txn_cid": 1})
	cursor, err := s.claimsCol.Find(ctx, bson.M{"org_key": orgKey}, findOpts)
	if err != nil {
		return 0, 0, status.Errorf(codes.Internal, "querying claims: %v", err)
	}
	defer cursor.Close(ctx)
	var cids []string
	for cursor.Next(ctx) {
		elements, err := cursor.Current.Elements()
		if err != nil {
			return 0, 0, err
		}
		for _, e := range elements {
			if e.Key() == "txn_cid" {
				cids = append(cids, e.Value().StringValue())
			}
		}
	}

	if len(cids) == 0 {
		return 0, 0, nil
	}

	res, err := s.sc.ListTxns(ctx, sendfil.ListTxnsMessageCids(cids))
	if err != nil {
		return 0, 0, err
	}

	totalPending := int64(0)
	totalActive := int64(0)
	for _, txn := range res {
		switch txn.MessageState {
		case sendfilpb.MessageState_MESSAGE_STATE_PENDING:
			totalPending += txn.AmountNanoFil
		case sendfilpb.MessageState_MESSAGE_STATE_ACTIVE:
			totalActive += txn.AmountNanoFil
		}
	}

	return totalPending, totalActive, nil
}

func (s *Service) getReward(ctx context.Context, orgKey string, t pb.RewardType) (*reward, error) {
	filter := bson.M{"org_key": orgKey, "type": t}
	res := s.rewardsCol.FindOne(ctx, filter)
	if res.Err() != nil {
		return nil, res.Err()
	}
	var r reward
	if err := res.Decode(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

func toPbReward(rec *reward) *pb.Reward {
	res := &pb.Reward{
		OrgKey:            rec.OrgKey,
		DevKey:            rec.DevKey,
		Type:              rec.Type,
		Factor:            rec.Factor,
		BaseNanoFilReward: rec.BaseNanoFILReward,
		CreatedAt:         timestamppb.New(rec.CreatedAt),
	}
	return res
}

func toPbClaim(rec *claim, txn *sendfilpb.Txn) *pb.Claim {
	res := &pb.Claim{
		Id:             rec.ID.Hex(),
		OrgKey:         rec.OrgKey,
		ClaimedBy:      rec.ClaimedBy,
		AmountNanoFil:  rec.AmountNanoFil,
		State:          txn.MessageState,
		TxnCid:         rec.TxnCid,
		FailureMessage: txn.FailureMsg,
		CreatedAt:      timestamppb.New(rec.CreatedAt),
		UpdatedAt:      txn.UpdatedAt,
	}
	return res
}

func ensureKeyRewardCache(keyCache map[string]map[pb.RewardType]struct{}, key string) {
	if _, exists := keyCache[key]; !exists {
		keyCache[key] = map[pb.RewardType]struct{}{}
	}
}
