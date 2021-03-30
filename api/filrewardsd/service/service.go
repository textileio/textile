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
	"github.com/textileio/textile/v2/api/filrewardsd/service/interfaces"
	sendfil "github.com/textileio/textile/v2/api/sendfild/client"
	sendfilpb "github.com/textileio/textile/v2/api/sendfild/pb"
	mdb "github.com/textileio/textile/v2/mongodb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	listMaxPageSize = 100
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

type orgKeyLock string

func (l orgKeyLock) Key() string {
	return string(l)
}

var _ nutil.SemaphoreKey = (*orgKeyLock)(nil)

var _ pb.FilRewardsServiceServer = (*Service)(nil)

type Service struct {
	Config
	rewardsCacheOrg   map[string]map[pb.RewardType]struct{}
	rewardsCacheDev   map[string]map[pb.RewardType]struct{}
	baseNanoFILReward int64
	server            *grpc.Server
	semaphores        *nutil.SemaphorePool
	mainCtxCancel     context.CancelFunc
}

type Config struct {
	Listener          net.Listener
	RewardStore       interfaces.RewardStore
	ClaimStore        interfaces.ClaimStore
	AccountStore      interfaces.AccountStore
	Analytics         interfaces.Analytics
	Sendfil           interfaces.SendFil
	Powergate         interfaces.Powergate
	BaseNanoFILReward int64
	FundingAddr       string
	Debug             bool
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

	// Populate caches.
	all, err := config.RewardStore.All(ctx)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("querying rewards to populate cache: %s", err)
	}
	cacheOrg := make(map[string]map[pb.RewardType]struct{})
	cacheDev := make(map[string]map[pb.RewardType]struct{})
	for _, rec := range all {
		ensureKeyRewardCache(cacheOrg, rec.OrgKey)
		ensureKeyRewardCache(cacheDev, rec.DevKey)
		cacheOrg[rec.OrgKey][rec.Type] = struct{}{}
		cacheDev[rec.DevKey][rec.Type] = struct{}{}
	}

	s := &Service{
		Config:            config,
		rewardsCacheOrg:   cacheOrg,
		rewardsCacheDev:   cacheDev,
		baseNanoFILReward: config.BaseNanoFILReward,
		semaphores:        nutil.NewSemaphorePool(1),
		mainCtxCancel:     cancel,
	}

	s.server = grpc.NewServer()
	go func() {
		pb.RegisterFilRewardsServiceServer(s.server, s)
		if err := s.server.Serve(s.Listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
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

	r, err := s.RewardStore.New(ctx, req.OrgKey, req.DevKey, t, rewardTypeMeta[t].factor, s.baseNanoFILReward)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating new reward: %v", err)
	}

	s.rewardsCacheOrg[req.OrgKey][t] = struct{}{}
	s.rewardsCacheDev[req.DevKey][t] = struct{}{}

	if err := s.Analytics.Track(
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

	return &pb.ProcessAnalyticsEventResponse{Reward: r}, nil
}

func (s *Service) ListRewards(ctx context.Context, req *pb.ListRewardsRequest) (*pb.ListRewardsResponse, error) {
	if req.PageSize > listMaxPageSize || req.PageSize == 0 {
		req.PageSize = listMaxPageSize
	}

	res, err := s.RewardStore.List(ctx, req)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "listing rewards: %v", err)
	}
	return &pb.ListRewardsResponse{Rewards: res}, nil
}

func (s *Service) Claim(ctx context.Context, req *pb.ClaimRequest) (*pb.ClaimResponse, error) {
	lck := s.semaphores.Get(orgKeyLock(req.OrgKey))
	lck.Acquire()
	defer lck.Release()

	totalNanoFilRewarded, err := s.RewardStore.TotalNanoFilRewarded(ctx, req.OrgKey)
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

	org, err := s.AccountStore.Get(ctx, pubKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "getting account: %v", err)
	}
	if org.Type != mdb.Org {
		return nil, status.Error(codes.InvalidArgument, "provided account key is not an org")
	}
	if org.PowInfo == nil {
		return nil, status.Error(codes.InvalidArgument, "no pow info for org")
	}

	ctxPow := context.WithValue(ctx, pow.AuthKey, org.PowInfo.Token)
	addrsRes, err := s.Powergate.Addresses(ctxPow)
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

	txn, err := s.Sendfil.SendFil(ctx, s.FundingAddr, toAddr, req.AmountNanoFil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "sending fil: %v", err)
	}

	c, err := s.ClaimStore.New(ctx, req.OrgKey, req.ClaimedBy, req.AmountNanoFil, txn.MessageCid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "creating claim: %v", err)
	}

	if err := s.Analytics.Track(
		ctx,
		c.OrgKey,
		analyticspb.AccountType_ACCOUNT_TYPE_ORG,
		analyticspb.Event_EVENT_FIL_CLAIM,
		analytics.WithProperties(map[string]interface{}{
			"id":              c.ID,
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

	conf := interfaces.ListConfig{
		OrgKeyFilter:    req.OrgKeyFilter,
		ClaimedByFilter: req.ClaimedByFilter,
		Ascending:       req.Ascending,
		PageSize:        req.PageSize,
		Page:            req.Page,
	}

	claims, err := s.ClaimStore.List(ctx, conf)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "querying claims: %v", err)
	}
	if len(claims) == 0 {
		return &pb.ListClaimsResponse{}, nil
	}

	var cids []string
	for _, claim := range claims {
		cids = append(cids, claim.TxnCid)
	}

	txns, err := s.Sendfil.ListTxns(ctx, sendfil.ListTxnsMessageCids(cids))
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
		pbClaims = append(pbClaims, toPbClaim(claim, txn))
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

	totalNanoFilRewarded, err := s.RewardStore.TotalNanoFilRewarded(ctx, req.OrgKey)
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

	return nil
}

func (s *Service) totalNanoFilClaimed(ctx context.Context, orgKey string) (int64, int64, error) {
	claims, err := s.ClaimStore.List(ctx, interfaces.ListConfig{OrgKeyFilter: orgKey})
	if err != nil {
		return 0, 0, status.Errorf(codes.Internal, "listing claims: %v", err)
	}

	if len(claims) == 0 {
		return 0, 0, nil
	}

	var cids []string
	for _, claim := range claims {
		cids = append(cids, claim.TxnCid)
	}

	res, err := s.Sendfil.ListTxns(ctx, sendfil.ListTxnsMessageCids(cids))
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

func toPbClaim(claim *interfaces.Claim, txn *sendfilpb.Txn) *pb.Claim {
	res := &pb.Claim{
		Id:             claim.ID,
		OrgKey:         claim.OrgKey,
		ClaimedBy:      claim.ClaimedBy,
		AmountNanoFil:  claim.AmountNanoFil,
		State:          txn.MessageState,
		TxnCid:         claim.TxnCid,
		FailureMessage: txn.FailureMsg,
		CreatedAt:      timestamppb.New(claim.CreatedAt),
		UpdatedAt:      txn.UpdatedAt,
	}
	return res
}

func ensureKeyRewardCache(keyCache map[string]map[pb.RewardType]struct{}, key string) {
	if _, exists := keyCache[key]; !exists {
		keyCache[key] = map[pb.RewardType]struct{}{}
	}
}
