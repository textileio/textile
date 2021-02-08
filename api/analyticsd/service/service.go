package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/gogo/status"
	logging "github.com/ipfs/go-log/v2"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/analyticsd/pb"
	filrewards "github.com/textileio/textile/v2/api/filrewardsd/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	segment "gopkg.in/segmentio/analytics-go.v3"
)

var (
	log = logging.Logger("analytics")
)

type Service struct {
	segment segment.Client
	prefix  string
	frc     *filrewards.Client
	debug   bool
	server  *grpc.Server
}

var _ pb.AnalyticsServiceServer = (*Service)(nil)

type Config struct {
	Listener       net.Listener
	SegmentAPIKey  string
	SegmentPrefix  string
	FilrewardsAddr string
	Debug          bool
}

func New(conf Config) (*Service, error) {
	if conf.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"analytics": logging.LevelDebug,
		}); err != nil {
			return nil, fmt.Errorf("setting log levels: %v", err)
		}
	}

	config := segment.Config{
		Verbose: conf.Debug,
	}
	segment, err := segment.NewWithConfig(conf.SegmentAPIKey, config)
	if err != nil {
		return nil, fmt.Errorf("creating segment client: %v", err)
	}

	s := &Service{
		segment: segment,
		prefix:  conf.SegmentPrefix,
		debug:   conf.Debug,
	}

	if conf.FilrewardsAddr != "" {
		s.frc, err = filrewards.New(conf.FilrewardsAddr)
		if err != nil {
			return nil, fmt.Errorf("creating filrewards client: %v", err)
		}
	}

	s.server = grpc.NewServer()
	go func() {
		pb.RegisterAnalyticsServiceServer(s.server, s)
		if err := s.server.Serve(conf.Listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Errorf("serve error: %v", err)
		}
	}()

	return s, nil
}

func (s *Service) Identify(ctx context.Context, req *pb.IdentifyRequest) (*pb.IdentifyResponse, error) {
	if req.AccountType == 2 {
		return &pb.IdentifyResponse{}, nil // Or should we return an error?
	}
	traits := segment.NewTraits()
	traits.Set("account_type", req.AccountType)
	traits.Set(s.prefix+"signup", "true")
	if req.Email != "" {
		traits.SetEmail(req.Email)
	}
	for key, value := range req.Properties.AsMap() {
		traits.Set(key, value)
	}
	err := s.segment.Enqueue(segment.Identify{
		UserId: req.Key,
		Traits: traits,
		Context: &segment.Context{
			Extra: map[string]interface{}{
				"active": req.Active,
			},
		},
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "enqueuing segment identify: %v", err)
	}
	return &pb.IdentifyResponse{}, nil
}

func (s *Service) Track(ctx context.Context, req *pb.TrackRequest) (*pb.TrackResponse, error) {
	if req.AccountType == 2 {
		return &pb.TrackResponse{}, nil // Or should we return an error?
	}
	props := segment.NewProperties()
	for key, value := range req.Properties.AsMap() {
		props.Set(key, value)
	}

	if err := s.segment.Enqueue(segment.Track{
		UserId:     req.Key,
		Event:      eventToString(req.Event),
		Properties: props,
		Context: &segment.Context{
			Extra: map[string]interface{}{
				"active": req.Active,
			},
		},
	}); err != nil {
		return nil, status.Errorf(codes.Internal, "enqueuing segment track: %v", err)
	}

	orgKey := ""
	devKey := ""
	if req.AccountType == pb.AccountType_ACCOUNT_TYPE_ORG {
		orgKey = req.Key
		devKey, _ = req.Properties.AsMap()["member"].(string)
	} else if req.AccountType == pb.AccountType_ACCOUNT_TYPE_DEV {
		devKey = req.Key
		orgKey, _ = req.Properties.AsMap()["org_key"].(string)
	}

	// Don't process the events that are generated by s.frc.ProcessAnalyticsEvent
	// when a reward or claim is created and sent here for analytics tracking.
	// Also be sure that we at least have an org key since that is required.
	if s.frc != nil && req.Event != pb.Event_EVENT_FIL_REWARD && req.Event != pb.Event_EVENT_FIL_CLAIM && req.Event != pb.Event_EVENT_FIL_FINALIZE_CLAIM && orgKey != "" {
		var opts []filrewards.ProcessAnalyticsEventOption
		if devKey != "" {
			opts = append(opts, filrewards.ProcessAnalyticsEventDevKey(devKey))
		}
		if _, err := s.frc.ProcessAnalyticsEvent(ctx, orgKey, req.Event, opts...); err != nil {
			return nil, status.Errorf(codes.Internal, "processing analytics event for filrewards: %v", err)
		}
	}

	return &pb.TrackResponse{}, nil
}

func (s *Service) Close() {
	if err := s.segment.Close(); err != nil {
		log.Errorf("closing segment client: %s", err)
	}
	log.Info("segment client disconnected")

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

func eventToString(e pb.Event) string {
	switch e {
	case pb.Event_EVENT_SIGN_IN:
		return "signin"
	case pb.Event_EVENT_ACCOUNT_DESTROYED:
		return "account_destroyed"
	case pb.Event_EVENT_KEY_ACCOUNT_CREATED:
		return "key_account_created"
	case pb.Event_EVENT_KEY_USER_CREATED:
		return "key_user_created"
	case pb.Event_EVENT_ORG_CREATED:
		return "org_created"
	case pb.Event_EVENT_ORG_LEAVE:
		return "org_leave"
	case pb.Event_EVENT_ORG_DESTROYED:
		return "org_destroyed"
	case pb.Event_EVENT_ORG_INVITE_CREATED:
		return "org_invite_created"
	case pb.Event_EVENT_GRACE_PERIOD_START:
		return "grace_period_start"
	case pb.Event_EVENT_GRACE_PERIOD_END:
		return "grace_period_end"
	case pb.Event_EVENT_BILLING_SETUP:
		return "billing_setup"
	case pb.Event_EVENT_BUCKET_CREATED:
		return "bucket_created"
	case pb.Event_EVENT_BUCKET_ARCHIVE_CREATED:
		return "bucket_archive_created"
	case pb.Event_EVENT_MAILBOX_CREATED:
		return "mailbox_created"
	case pb.Event_EVENT_THREAD_DB_CREATED:
		return "threaddb_created"
	case pb.Event_EVENT_FIL_REWARD:
		return "fil_reward"
	case pb.Event_EVENT_FIL_CLAIM:
		return "fil_claim"
	case pb.Event_EVENT_FIL_FINALIZE_CLAIM:
		return "fil_finalize_claim"
	default:
		return fmt.Sprintf("%d", int(e))
	}
}
