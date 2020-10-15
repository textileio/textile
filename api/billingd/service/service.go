package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net"
	"time"

	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	stripe "github.com/stripe/stripe-go/v72"
	stripec "github.com/stripe/stripe-go/v72/client"
	"github.com/textileio/go-threads/util"
	pb "github.com/textileio/textile/v2/api/billingd/pb"
	"github.com/textileio/textile/v2/api/common"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc"
)

const (
	mib = 1024 * 1024
	gib = 1024 * mib

	StoredDataUnitSize     = 5 * gib / 100
	NetworkEgressUnitSize  = 10 * gib / 100
	InstanceReadsUnitSize  = 10000
	InstanceWritesUnitSize = 5000

	FreeStoredDataUnits    = 100                               // 5 Gib
	FreeNetworkEgressUnits = 500 * mib / NetworkEgressUnitSize // 500 Mib per day
	FreeInstanceReadUnits  = 1                                 // 10,000 per day
	FreeInstanceWriteUnits = 1                                 // 5,000 per day
)

var (
	log = logging.Logger("billing")

	// ErrExceedsFreeUnits indicates the requested operation exceeds the free unit quota.
	ErrExceedsFreeUnits = errors.New("request exceeds free unit quota")
)

type Service struct {
	config Config
	server *grpc.Server
	stripe *stripec.API
	db     *mongo.Collection
}

var _ pb.APIServiceServer = (*Service)(nil)

type Config struct {
	ListenAddr ma.Multiaddr

	StripeAPIURL           string
	StripeAPIKey           string
	StripeSessionReturnURL string

	DBURI  string
	DBName string

	StoredDataPriceID     string
	NetworkEgressPriceID  string
	InstanceReadsPriceID  string
	InstanceWritesPriceID string

	Debug bool
}

func NewService(ctx context.Context, config Config, createPrices bool) (*Service, error) {
	if config.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"billing": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	sc, err := common.NewStripeClient(config.StripeAPIURL, config.StripeAPIKey)
	if err != nil {
		return nil, err
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.DBURI))
	if err != nil {
		return nil, err
	}

	customers := client.Database(config.DBName).Collection("customers")
	indexes, err := customers.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{"email", 1}},
		},
	})
	if err != nil {
		return nil, err
	}
	for _, index := range indexes {
		log.Infof("created index: %s", index)
	}

	s := &Service{
		config: config,
		db:     customers,
		stripe: sc,
	}

	if createPrices {
		s.config.StoredDataPriceID, err = createStoredData(sc)
		if err != nil {
			return nil, err
		}
		s.config.NetworkEgressPriceID, err = createNetworkEgress(sc)
		if err != nil {
			return nil, err
		}
		s.config.InstanceReadsPriceID, err = createInstanceReads(sc)
		if err != nil {
			return nil, err
		}
		s.config.InstanceWritesPriceID, err = createInstanceWrites(sc)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func (s *Service) Start() error {
	s.server = grpc.NewServer()
	target, err := util.TCPAddrFromMultiAddr(s.config.ListenAddr)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", target)
	if err != nil {
		return err
	}
	go func() {
		pb.RegisterAPIServiceServer(s.server, s)
		if err := s.server.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			log.Errorf("serve error: %v", err)
		}
	}()
	return nil
}

func (s *Service) Stop(force bool) error {
	if force {
		s.server.Stop()
	} else {
		s.server.GracefulStop()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return s.db.Database().Client().Disconnect(ctx)
}

type Customer struct {
	ID             string `bson:"_id"`
	Email          string `bson:"email"`
	SubscriptionID string `bson:"subscription_id"`
	Billable       bool   `json:"billable"`

	StoredData     StoredData     `bson:"stored_data"`
	NetworkEgress  NetworkEgress  `bson:"network_egress"`
	InstanceReads  InstanceReads  `bson:"instance_reads"`
	InstanceWrites InstanceWrites `bson:"instance_writes"`

	CreatedAt int64 `bson:"created_at"`
}

type StoredData struct {
	ItemID    string `bson:"item_id"`
	TotalSize int64  `bson:"total_size"`
	Units     int64  `bson:"units"`
}

type NetworkEgress struct {
	ItemID   string `bson:"item_id"`
	SubUnits int64  `bson:"sub_units"`
}

type InstanceReads struct {
	ItemID   string `bson:"item_id"`
	SubUnits int64  `bson:"sub_units"`
}

type InstanceWrites struct {
	ItemID   string `bson:"item_id"`
	SubUnits int64  `bson:"sub_units"`
}

func (s *Service) CheckHealth(_ context.Context, _ *pb.CheckHealthRequest) (*pb.CheckHealthResponse, error) {
	log.Debugf("health check okay")
	return &pb.CheckHealthResponse{}, nil
}

func (s *Service) CreateCustomer(ctx context.Context, req *pb.CreateCustomerRequest) (
	*pb.CreateCustomerResponse, error) {
	customer, err := s.stripe.Customers.New(&stripe.CustomerParams{
		Email: stripe.String(req.Email),
	})
	if err != nil {
		return nil, err
	}
	subscription, err := s.stripe.Subscriptions.New(&stripe.SubscriptionParams{
		Customer: stripe.String(customer.ID),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: stripe.String(s.config.StoredDataPriceID),
			},
			{
				Price: stripe.String(s.config.NetworkEgressPriceID),
			},
			{
				Price: stripe.String(s.config.InstanceReadsPriceID),
			},
			{
				Price: stripe.String(s.config.InstanceWritesPriceID),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	doc := &Customer{
		ID:             customer.ID,
		Email:          customer.Email,
		SubscriptionID: subscription.ID,
		CreatedAt:      time.Now().UnixNano(),
	}
	for _, item := range subscription.Items.Data {
		switch item.Price.ID {
		case s.config.StoredDataPriceID:
			doc.StoredData.ItemID = item.ID
		case s.config.NetworkEgressPriceID:
			doc.NetworkEgress.ItemID = item.ID
		case s.config.InstanceReadsPriceID:
			doc.InstanceReads.ItemID = item.ID
		case s.config.InstanceWritesPriceID:
			doc.InstanceWrites.ItemID = item.ID
		}
	}

	if _, err := s.db.InsertOne(ctx, doc); err != nil {
		return nil, err
	}
	log.Debugf("created customer %s", customer.ID)
	return &pb.CreateCustomerResponse{CustomerId: customer.ID}, nil
}

func (s *Service) GetCustomer(_ context.Context, req *pb.GetCustomerRequest) (
	*pb.GetCustomerResponse, error) {
	customer, err := s.stripe.Customers.Get(req.CustomerId, nil)
	if err != nil {
		return nil, err
	}
	log.Debugf("got customer %s", customer.ID)
	return &pb.GetCustomerResponse{
		Balance:    customer.Balance,
		Billable:   !customer.Deleted && customer.DefaultSource != nil,
		Delinquent: customer.Delinquent,
	}, nil
}

func (s *Service) GetCustomerSession(_ context.Context, req *pb.GetCustomerSessionRequest) (
	*pb.GetCustomerSessionResponse, error) {
	session, err := s.stripe.BillingPortalSessions.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(req.CustomerId),
		ReturnURL: stripe.String(s.config.StripeSessionReturnURL),
	})
	if err != nil {
		return nil, err
	}
	return &pb.GetCustomerSessionResponse{
		Url: session.URL,
	}, nil
}

func (s *Service) DeleteCustomer(ctx context.Context, req *pb.DeleteCustomerRequest) (
	*pb.DeleteCustomerResponse, error) {
	if _, err := s.stripe.Customers.Del(req.CustomerId, nil); err != nil {
		return nil, err
	}
	if _, err := s.db.DeleteOne(ctx, bson.M{"_id": req.CustomerId}); err != nil {
		return nil, err
	}
	log.Debugf("deleted customer %s", req.CustomerId)
	return &pb.DeleteCustomerResponse{}, nil
}

func (s *Service) AddCard(ctx context.Context, req *pb.AddCardRequest) (*pb.AddCardResponse, error) {
	crd, err := s.stripe.Cards.New(&stripe.CardParams{
		Customer: stripe.String(req.CustomerId),
		Token:    stripe.String(req.Token),
	})
	if err != nil {
		return nil, err
	}
	_, err = s.db.UpdateOne(ctx, bson.M{"_id": req.CustomerId}, bson.M{"$set": bson.M{"billable": true}})
	if err != nil {
		return nil, err
	}

	log.Debugf("added card %s for %s", crd.ID, req.CustomerId)
	return &pb.AddCardResponse{}, nil
}

func (s *Service) IncStoredData(ctx context.Context, req *pb.IncStoredDataRequest) (*pb.IncStoredDataResponse, error) {
	sess, err := s.db.Database().Client().StartSession()
	if err != nil {
		return nil, err
	}
	defer sess.EndSession(ctx)
	if err = sess.StartTransaction(); err != nil {
		return nil, err
	}
	res := &pb.IncStoredDataResponse{}
	err = mongo.WithSession(ctx, sess, func(sctx mongo.SessionContext) error {
		r := s.db.FindOne(sctx, bson.M{"_id": req.CustomerId})
		if r.Err() != nil {
			return r.Err()
		}
		var doc Customer
		if err := r.Decode(&doc); err != nil {
			return err
		}
		res.TotalSize = doc.StoredData.TotalSize + req.IncSize
		if res.TotalSize < 0 {
			res.TotalSize = 0
		}
		update := bson.M{"stored_data.total_size": res.TotalSize}
		res.Units = int64(math.Round(float64(res.TotalSize) / float64(StoredDataUnitSize)))
		if res.Units > FreeStoredDataUnits && !doc.Billable {
			return ErrExceedsFreeUnits
		}
		if res.Units != doc.StoredData.Units {
			if _, err := s.stripe.UsageRecords.New(&stripe.UsageRecordParams{
				SubscriptionItem: stripe.String(doc.StoredData.ItemID),
				Quantity:         stripe.Int64(res.Units),
				Timestamp:        stripe.Int64(time.Now().Unix()),
				Action:           stripe.String(stripe.UsageRecordActionSet),
			}); err != nil {
				return err
			}
			update["stored_data.units"] = res.Units
		}
		if _, err := s.db.UpdateOne(sctx, bson.M{"_id": req.CustomerId}, bson.M{"$set": update}); err != nil {
			return err
		}
		return sess.CommitTransaction(sctx)
	})
	if err != nil {
		if err := sess.AbortTransaction(ctx); err != nil {
			log.Errorf("aborting txn: %v", err)
		}
		return nil, err
	}
	log.Debugf(
		"%s period data: units=%d total_size=%d",
		req.CustomerId,
		res.Units,
		res.TotalSize,
	)
	return res, nil
}

func (s *Service) GetStoredData(ctx context.Context, req *pb.GetStoredDataRequest) (
	*pb.GetStoredDataResponse, error) {
	r := s.db.FindOne(ctx, bson.M{"_id": req.CustomerId})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	if err := r.Decode(&doc); err != nil {
		return nil, err
	}
	sum, err := s.getPeriodUsageItem(doc.StoredData.ItemID)
	if err != nil {
		return nil, err
	}
	res := &pb.GetStoredDataResponse{
		Units:     sum.TotalUsage,
		TotalSize: doc.StoredData.TotalSize,
		FreeSize:  (FreeStoredDataUnits * StoredDataUnitSize) - doc.StoredData.TotalSize,
		Period: &pb.UsagePeriod{
			Start: sum.Period.Start,
			End:   sum.Period.End,
		},
	}
	log.Debugf("got stored data for %s", req.CustomerId)
	return res, nil
}

func (s *Service) IncNetworkEgress(ctx context.Context, req *pb.IncNetworkEgressRequest) (
	*pb.IncNetworkEgressResponse, error) {
	sess, err := s.db.Database().Client().StartSession()
	if err != nil {
		return nil, err
	}
	defer sess.EndSession(ctx)
	if err = sess.StartTransaction(); err != nil {
		return nil, err
	}
	res := &pb.IncNetworkEgressResponse{}
	err = mongo.WithSession(ctx, sess, func(sctx mongo.SessionContext) error {
		r := s.db.FindOne(sctx, bson.M{"_id": req.CustomerId})
		if r.Err() != nil {
			return r.Err()
		}
		var doc Customer
		if err := r.Decode(&doc); err != nil {
			return err
		}
		sum, err := s.getPeriodUsageItem(doc.NetworkEgress.ItemID)
		if err != nil {
			return err
		}
		res.SubUnits = doc.NetworkEgress.SubUnits + req.IncSize
		if res.SubUnits >= NetworkEgressUnitSize {
			res.AddedUnits = res.SubUnits / NetworkEgressUnitSize
			if res.AddedUnits+sum.TotalUsage > FreeNetworkEgressUnits && !doc.Billable {
				return ErrExceedsFreeUnits
			}
			res.SubUnits = res.SubUnits % NetworkEgressUnitSize
			_, err := s.stripe.UsageRecords.New(&stripe.UsageRecordParams{
				SubscriptionItem: stripe.String(doc.NetworkEgress.ItemID),
				Quantity:         stripe.Int64(res.AddedUnits),
				Timestamp:        stripe.Int64(time.Now().Unix()),
				Action:           stripe.String(stripe.UsageRecordActionIncrement),
			})
			if err != nil {
				return err
			}
		}
		if _, err := s.db.UpdateOne(sctx, bson.M{"_id": req.CustomerId}, bson.M{
			"$set": bson.M{"network_egress.sub_units": res.SubUnits},
		}); err != nil {
			return err
		}
		return sess.CommitTransaction(sctx)
	})
	if err != nil {
		if err := sess.AbortTransaction(ctx); err != nil {
			log.Errorf("aborting txn: %v", err)
		}
		return nil, err
	}
	log.Debugf("%s period egress: added_units=%d sub_units=%d", req.CustomerId, res.AddedUnits, res.SubUnits)
	return res, nil
}

func (s *Service) GetNetworkEgress(ctx context.Context, req *pb.GetNetworkEgressRequest) (
	*pb.GetNetworkEgressResponse, error) {
	r := s.db.FindOne(ctx, bson.M{"_id": req.CustomerId})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	if err := r.Decode(&doc); err != nil {
		return nil, err
	}
	sum, err := s.getPeriodUsageItem(doc.NetworkEgress.ItemID)
	if err != nil {
		return nil, err
	}
	res := &pb.GetNetworkEgressResponse{
		Units:     sum.TotalUsage,
		SubUnits:  doc.NetworkEgress.SubUnits,
		FreeUnits: FreeNetworkEgressUnits - sum.TotalUsage,
		Period: &pb.UsagePeriod{
			Start: sum.Period.Start,
			End:   sum.Period.End,
		},
	}
	log.Debugf("got network egress for %s", req.CustomerId)
	return res, nil
}

func (s *Service) IncInstanceReads(ctx context.Context, req *pb.IncInstanceReadsRequest) (
	*pb.IncInstanceReadsResponse, error) {
	sess, err := s.db.Database().Client().StartSession()
	if err != nil {
		return nil, err
	}
	defer sess.EndSession(ctx)
	if err = sess.StartTransaction(); err != nil {
		return nil, err
	}
	res := &pb.IncInstanceReadsResponse{}
	err = mongo.WithSession(ctx, sess, func(sctx mongo.SessionContext) error {
		r := s.db.FindOne(sctx, bson.M{"_id": req.CustomerId})
		if r.Err() != nil {
			return r.Err()
		}
		var doc Customer
		if err := r.Decode(&doc); err != nil {
			return err
		}
		sum, err := s.getPeriodUsageItem(doc.InstanceReads.ItemID)
		if err != nil {
			return err
		}
		res.SubUnits = doc.InstanceReads.SubUnits + req.IncCount
		if res.SubUnits >= InstanceReadsUnitSize {
			res.AddedUnits = res.SubUnits / InstanceReadsUnitSize
			if res.AddedUnits+sum.TotalUsage > FreeInstanceReadUnits && !doc.Billable {
				return ErrExceedsFreeUnits
			}
			res.SubUnits = res.SubUnits % InstanceReadsUnitSize
			_, err := s.stripe.UsageRecords.New(&stripe.UsageRecordParams{
				SubscriptionItem: stripe.String(doc.InstanceReads.ItemID),
				Quantity:         stripe.Int64(res.AddedUnits),
				Timestamp:        stripe.Int64(time.Now().Unix()),
				Action:           stripe.String(stripe.UsageRecordActionIncrement),
			})
			if err != nil {
				return err
			}
		}
		if _, err := s.db.UpdateOne(sctx, bson.M{"_id": req.CustomerId}, bson.M{
			"$set": bson.M{"instance_reads.sub_units": res.SubUnits},
		}); err != nil {
			return err
		}
		return sess.CommitTransaction(sctx)
	})
	if err != nil {
		if err := sess.AbortTransaction(ctx); err != nil {
			log.Errorf("aborting txn: %v", err)
		}
		return nil, err
	}
	log.Debugf("%s period reads: added_units=%d sub_units=%d", req.CustomerId, res.AddedUnits, res.SubUnits)
	return res, nil
}

func (s *Service) GetInstanceReads(ctx context.Context, req *pb.GetInstanceReadsRequest) (
	*pb.GetInstanceReadsResponse, error) {
	r := s.db.FindOne(ctx, bson.M{"_id": req.CustomerId})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	if err := r.Decode(&doc); err != nil {
		return nil, err
	}
	sum, err := s.getPeriodUsageItem(doc.InstanceReads.ItemID)
	if err != nil {
		return nil, err
	}
	res := &pb.GetInstanceReadsResponse{
		Units:     sum.TotalUsage,
		SubUnits:  doc.InstanceReads.SubUnits,
		FreeUnits: FreeInstanceReadUnits - sum.TotalUsage,
		Period: &pb.UsagePeriod{
			Start: sum.Period.Start,
			End:   sum.Period.End,
		},
	}
	log.Debugf("got instance reads for %s", req.CustomerId)
	return res, nil
}

func (s *Service) IncInstanceWrites(ctx context.Context, req *pb.IncInstanceWritesRequest) (
	*pb.IncInstanceWritesResponse, error) {
	sess, err := s.db.Database().Client().StartSession()
	if err != nil {
		return nil, err
	}
	defer sess.EndSession(ctx)
	if err = sess.StartTransaction(); err != nil {
		return nil, err
	}
	res := &pb.IncInstanceWritesResponse{}
	err = mongo.WithSession(ctx, sess, func(sctx mongo.SessionContext) error {
		r := s.db.FindOne(sctx, bson.M{"_id": req.CustomerId})
		if r.Err() != nil {
			return r.Err()
		}
		var doc Customer
		if err := r.Decode(&doc); err != nil {
			return err
		}
		sum, err := s.getPeriodUsageItem(doc.InstanceWrites.ItemID)
		if err != nil {
			return err
		}
		res.SubUnits = doc.InstanceWrites.SubUnits + req.IncCount
		if res.SubUnits >= InstanceWritesUnitSize {
			res.AddedUnits = res.SubUnits / InstanceWritesUnitSize
			if res.AddedUnits+sum.TotalUsage > FreeInstanceWriteUnits && !doc.Billable {
				return ErrExceedsFreeUnits
			}
			res.SubUnits = res.SubUnits % InstanceWritesUnitSize
			_, err := s.stripe.UsageRecords.New(&stripe.UsageRecordParams{
				SubscriptionItem: stripe.String(doc.InstanceWrites.ItemID),
				Quantity:         stripe.Int64(res.AddedUnits),
				Timestamp:        stripe.Int64(time.Now().Unix()),
				Action:           stripe.String(stripe.UsageRecordActionIncrement),
			})
			if err != nil {
				return err
			}
		}
		if _, err := s.db.UpdateOne(sctx, bson.M{"_id": req.CustomerId}, bson.M{
			"$set": bson.M{"instance_writes.sub_units": res.SubUnits},
		}); err != nil {
			return err
		}
		return sess.CommitTransaction(sctx)
	})
	if err != nil {
		if err := sess.AbortTransaction(ctx); err != nil {
			log.Errorf("aborting txn: %v", err)
		}
		return nil, err
	}
	log.Debugf("%s period writes: added_units=%d sub_units=%d", req.CustomerId, res.AddedUnits, res.SubUnits)
	return res, nil
}

func (s *Service) GetInstanceWrites(ctx context.Context, req *pb.GetInstanceWritesRequest) (
	*pb.GetInstanceWritesResponse, error) {
	r := s.db.FindOne(ctx, bson.M{"_id": req.CustomerId})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	if err := r.Decode(&doc); err != nil {
		return nil, err
	}
	sum, err := s.getPeriodUsageItem(doc.InstanceWrites.ItemID)
	if err != nil {
		return nil, err
	}
	res := &pb.GetInstanceWritesResponse{
		Units:     sum.TotalUsage,
		SubUnits:  doc.InstanceWrites.SubUnits,
		FreeUnits: FreeInstanceWriteUnits - sum.TotalUsage,
		Period: &pb.UsagePeriod{
			Start: sum.Period.Start,
			End:   sum.Period.End,
		},
	}
	log.Debugf("got instance writes for %s", req.CustomerId)
	return res, nil
}

func (s *Service) getPeriodUsageItem(id string) (sum *stripe.UsageRecordSummary, err error) {
	params := &stripe.UsageRecordSummaryListParams{
		SubscriptionItem: stripe.String(id),
	}
	params.Filters.AddFilter("limit", "", "1")
	i := s.stripe.UsageRecordSummaries.List(params)
	if i.Err() != nil {
		return nil, i.Err()
	}
	for i.Next() {
		sum = i.UsageRecordSummary()
	}
	if sum != nil && sum.Period != nil {
		return sum, nil
	}
	return nil, fmt.Errorf("subscription item %s not found", id)
}
