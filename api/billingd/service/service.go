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
	"github.com/textileio/textile/v2/api/billingd/common"
	"github.com/textileio/textile/v2/api/billingd/gateway"
	pb "github.com/textileio/textile/v2/api/billingd/pb"
	apicommon "github.com/textileio/textile/v2/api/common"
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

var log = logging.Logger("billing")

type Service struct {
	config  Config
	server  *grpc.Server
	stripe  *stripec.API
	db      *mongo.Collection
	gateway *gateway.Gateway
}

var _ pb.APIServiceServer = (*Service)(nil)

type Config struct {
	ListenAddr ma.Multiaddr

	StripeAPIURL           string
	StripeAPIKey           string
	StripeSessionReturnURL string
	StripeWebhookSecret    string

	DBURI  string
	DBName string

	GatewayHostAddr ma.Multiaddr

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

	sc, err := apicommon.NewStripeClient(config.StripeAPIURL, config.StripeAPIKey)
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
	s.gateway, err = gateway.NewGateway(gateway.Config{
		Addr:                config.GatewayHostAddr,
		APIAddr:             config.ListenAddr,
		StripeWebhookSecret: config.StripeWebhookSecret,
		Debug:               config.Debug,
	})
	if err != nil {
		return nil, err
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
	s.gateway.Start()
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
	if err := s.gateway.Stop(); err != nil {
		return err
	}
	return s.db.Database().Client().Disconnect(ctx)
}

type Customer struct {
	ID         string `bson:"_id"`
	Email      string `bson:"email"`
	Status     string `bson:"status"`
	Balance    int64  `bson:"balance"`
	Billable   bool   `bson:"billable"`
	Delinquent bool   `bson:"delinquent"`
	CreatedAt  int64  `bson:"created_at"`

	Period         UsagePeriod    `bson:"period"`
	StoredData     StoredData     `bson:"stored_data"`
	NetworkEgress  NetworkEgress  `bson:"network_egress"`
	InstanceReads  InstanceReads  `bson:"instance_reads"`
	InstanceWrites InstanceWrites `bson:"instance_writes"`
}

type UsagePeriod struct {
	Start int64 `bson:"start"`
	End   int64 `bson:"end"`
}

type StoredData struct {
	ItemID    string `bson:"item_id"`
	Units     int64  `bson:"units"`
	TotalSize int64  `bson:"total_size"`
}

type NetworkEgress struct {
	ItemID   string `bson:"item_id"`
	Units    int64  `bson:"units"`
	SubUnits int64  `bson:"sub_units"`
}

type InstanceReads struct {
	ItemID   string `bson:"item_id"`
	Units    int64  `bson:"units"`
	SubUnits int64  `bson:"sub_units"`
}

type InstanceWrites struct {
	ItemID   string `bson:"item_id"`
	Units    int64  `bson:"units"`
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
	doc := &Customer{
		ID:        customer.ID,
		Email:     customer.Email,
		CreatedAt: time.Now().Unix(),
	}
	if err := s.createSubscription(doc); err != nil {
		return nil, err
	}
	if _, err := s.db.InsertOne(ctx, doc); err != nil {
		return nil, err
	}
	log.Debugf("created customer %s", customer.ID)
	return &pb.CreateCustomerResponse{CustomerId: customer.ID}, nil
}

func (s *Service) createSubscription(cus *Customer) error {
	sub, err := s.stripe.Subscriptions.New(&stripe.SubscriptionParams{
		Customer: stripe.String(cus.ID),
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
		return err
	}
	cus.Status = string(sub.Status)
	cus.Period = UsagePeriod{
		Start: sub.CurrentPeriodStart,
		End:   sub.CurrentPeriodEnd,
	}
	for _, item := range sub.Items.Data {
		switch item.Price.ID {
		case s.config.StoredDataPriceID:
			cus.StoredData.ItemID = item.ID // Retain existing units since this is "last ever" usage
		case s.config.NetworkEgressPriceID:
			cus.NetworkEgress = NetworkEgress{ItemID: item.ID}
		case s.config.InstanceReadsPriceID:
			cus.InstanceReads = InstanceReads{ItemID: item.ID}
		case s.config.InstanceWritesPriceID:
			cus.InstanceWrites = InstanceWrites{ItemID: item.ID}
		}
	}
	return nil
}

func (s *Service) GetCustomer(ctx context.Context, req *pb.GetCustomerRequest) (
	*pb.GetCustomerResponse, error) {
	r := s.db.FindOne(ctx, bson.M{"_id": req.CustomerId})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	if err := r.Decode(&doc); err != nil {
		return nil, err
	}
	log.Debugf("got customer %s", doc.ID)
	return &pb.GetCustomerResponse{
		Email:      doc.Email,
		Status:     doc.Status,
		Balance:    doc.Balance,
		Billable:   doc.Billable,
		Delinquent: doc.Delinquent,
		CreatedAt:  doc.CreatedAt,
		Period: &pb.UsagePeriod{
			Start: doc.Period.Start,
			End:   doc.Period.End,
		},
		StoredData: &pb.AggregateStoredDataUsage{
			Units:     doc.StoredData.Units,
			TotalSize: doc.StoredData.TotalSize,
			FreeSize:  (FreeStoredDataUnits * StoredDataUnitSize) - doc.StoredData.TotalSize,
		},
		NetworkEgress: &pb.AggregateSumUsage{
			Units:     doc.NetworkEgress.Units,
			SubUnits:  doc.NetworkEgress.SubUnits,
			FreeUnits: FreeNetworkEgressUnits - doc.NetworkEgress.Units,
		},
		InstanceReads: &pb.AggregateSumUsage{
			Units:     doc.InstanceReads.Units,
			SubUnits:  doc.InstanceReads.SubUnits,
			FreeUnits: FreeInstanceReadUnits - doc.InstanceReads.Units,
		},
		InstanceWrites: &pb.AggregateSumUsage{
			Units:     doc.InstanceWrites.Units,
			SubUnits:  doc.InstanceWrites.SubUnits,
			FreeUnits: FreeInstanceWriteUnits - doc.InstanceWrites.Units,
		},
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

func (s *Service) UpdateCustomer(ctx context.Context, req *pb.UpdateCustomerRequest) (
	*pb.UpdateCustomerResponse, error) {
	if _, err := s.db.UpdateOne(ctx, bson.M{"_id": req.CustomerId}, bson.M{
		"$set": bson.M{"balance": req.Balance, "billable": req.Billable, "delinquent": req.Delinquent},
	}); err != nil {
		return nil, err
	}
	log.Debugf("updated customer %s", req.CustomerId)
	return &pb.UpdateCustomerResponse{}, nil
}

func (s *Service) UpdateCustomerSubscription(ctx context.Context, req *pb.UpdateCustomerSubscriptionRequest) (
	*pb.UpdateCustomerSubscriptionResponse, error) {
	sess, err := s.db.Database().Client().StartSession()
	if err != nil {
		return nil, err
	}
	defer sess.EndSession(ctx)
	if err = sess.StartTransaction(); err != nil {
		return nil, err
	}
	err = mongo.WithSession(ctx, sess, func(sctx mongo.SessionContext) error {
		r := s.db.FindOneAndUpdate(sctx, bson.M{"_id": req.CustomerId}, bson.M{
			"$set": bson.M{
				"status":       req.Status,
				"period.start": req.Period.Start,
				"period.end":   req.Period.End,
			},
		})
		if r.Err() != nil {
			return r.Err()
		}
		var pre Customer
		if err := r.Decode(&pre); err != nil {
			return err
		}
		if pre.Period.End < req.Period.End {
			if _, err := s.db.UpdateOne(sctx, bson.M{"_id": req.CustomerId}, bson.M{
				"$set": bson.M{
					"network_egress.units":      0,
					"network_egress.sub_units":  0,
					"instance_reads.units":      0,
					"instance_reads.sub_units":  0,
					"instance_writes.units":     0,
					"instance_writes.sub_units": 0,
				}}); err != nil {
				return err
			}
		}
		return sess.CommitTransaction(sctx)
	})
	if err != nil {
		if err := sess.AbortTransaction(ctx); err != nil {
			log.Errorf("aborting txn: %v", err)
		}
		return nil, err
	}
	log.Debugf("updated subscription with status '%s' for %s", req.Status, req.CustomerId)
	return &pb.UpdateCustomerSubscriptionResponse{}, nil
}

func (s *Service) RecreateCustomerSubscription(ctx context.Context, req *pb.RecreateCustomerSubscriptionRequest) (
	*pb.RecreateCustomerSubscriptionResponse, error) {
	sess, err := s.db.Database().Client().StartSession()
	if err != nil {
		return nil, err
	}
	defer sess.EndSession(ctx)
	if err = sess.StartTransaction(); err != nil {
		return nil, err
	}
	err = mongo.WithSession(ctx, sess, func(sctx mongo.SessionContext) error {
		r := s.db.FindOne(sctx, bson.M{"_id": req.CustomerId})
		if r.Err() != nil {
			return r.Err()
		}
		var doc Customer
		if err := r.Decode(&doc); err != nil {
			return err
		}
		if err := common.StatusCheck(doc.Status); err == nil {
			return common.ErrSubscriptionExists
		} else if !errors.Is(err, common.ErrSubscriptionCanceled) {
			return err
		}
		if err := s.createSubscription(&doc); err != nil {
			return err
		}
		if _, err := s.db.UpdateOne(sctx, bson.M{"_id": req.CustomerId}, bson.M{
			"$set": bson.M{
				"status":          doc.Status,
				"period":          doc.Period,
				"stored_data":     doc.StoredData,
				"network_egress":  doc.NetworkEgress,
				"instance_reads":  doc.InstanceReads,
				"instance_writes": doc.InstanceWrites,
			}}); err != nil {
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
	log.Debugf("recreated subscription for %s", req.CustomerId)
	return &pb.RecreateCustomerSubscriptionResponse{}, nil
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
		if err := common.StatusCheck(doc.Status); err != nil {
			return err
		}
		totalSize := doc.StoredData.TotalSize + req.IncSize
		if totalSize < 0 {
			totalSize = 0
		}
		update := bson.M{"stored_data.total_size": totalSize}
		units := int64(math.Round(float64(totalSize) / float64(StoredDataUnitSize)))
		if units > FreeStoredDataUnits && !doc.Billable {
			return common.ErrExceedsFreeUnits
		}
		if units != doc.StoredData.Units {
			if _, err := s.stripe.UsageRecords.New(&stripe.UsageRecordParams{
				SubscriptionItem: stripe.String(doc.StoredData.ItemID),
				Quantity:         stripe.Int64(units),
				Timestamp:        stripe.Int64(time.Now().Unix()),
				Action:           stripe.String(stripe.UsageRecordActionSet),
			}); err != nil {
				return err
			}
			update["stored_data.units"] = units
		}
		if _, err := s.db.UpdateOne(sctx, bson.M{"_id": req.CustomerId}, bson.M{"$set": update}); err != nil {
			return err
		}
		res = &pb.IncStoredDataResponse{
			Period: &pb.UsagePeriod{
				Start: doc.Period.Start,
				End:   doc.Period.End,
			},
			StoredData: &pb.AggregateStoredDataUsage{
				Units:     units,
				TotalSize: totalSize,
				FreeSize:  (FreeStoredDataUnits * StoredDataUnitSize) - totalSize,
			},
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
		"%s period data: units=%d total_size=%d free_size=%d",
		req.CustomerId,
		res.StoredData.Units,
		res.StoredData.TotalSize,
		res.StoredData.FreeSize,
	)
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
		if err := common.StatusCheck(doc.Status); err != nil {
			return err
		}
		var units, subUnits int64
		subUnits = doc.NetworkEgress.SubUnits + req.IncSize
		if subUnits >= NetworkEgressUnitSize {
			addedUnits := subUnits / NetworkEgressUnitSize
			units = addedUnits + doc.NetworkEgress.Units
			if units > FreeNetworkEgressUnits && !doc.Billable {
				return common.ErrExceedsFreeUnits
			}
			subUnits = subUnits % NetworkEgressUnitSize
			_, err := s.stripe.UsageRecords.New(&stripe.UsageRecordParams{
				SubscriptionItem: stripe.String(doc.NetworkEgress.ItemID),
				Quantity:         stripe.Int64(units),
				Timestamp:        stripe.Int64(time.Now().Unix()),
				Action:           stripe.String(stripe.UsageRecordActionSet),
			})
			if err != nil {
				return err
			}
		}
		if _, err := s.db.UpdateOne(sctx, bson.M{"_id": req.CustomerId}, bson.M{
			"$set": bson.M{"network_egress.units": units, "network_egress.sub_units": subUnits},
		}); err != nil {
			return err
		}
		res = &pb.IncNetworkEgressResponse{
			Period: &pb.UsagePeriod{
				Start: doc.Period.Start,
				End:   doc.Period.End,
			},
			NetworkEgress: &pb.AggregateSumUsage{
				Units:     units,
				SubUnits:  subUnits,
				FreeUnits: FreeNetworkEgressUnits - units,
			},
		}
		return sess.CommitTransaction(sctx)
	})
	if err != nil {
		if err := sess.AbortTransaction(ctx); err != nil {
			log.Errorf("aborting txn: %v", err)
		}
		return nil, err
	}
	log.Debugf("%s period egress: units=%d sub_units=%d free_units=%d",
		req.CustomerId,
		res.NetworkEgress.Units,
		res.NetworkEgress.SubUnits,
		res.NetworkEgress.FreeUnits,
	)
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
		if err := common.StatusCheck(doc.Status); err != nil {
			return err
		}
		var units, subUnits int64
		subUnits = doc.InstanceReads.SubUnits + req.IncCount
		if subUnits >= InstanceReadsUnitSize {
			addedUnits := subUnits / InstanceReadsUnitSize
			units = addedUnits + doc.InstanceReads.Units
			if units > FreeInstanceReadUnits && !doc.Billable {
				return common.ErrExceedsFreeUnits
			}
			subUnits = subUnits % InstanceReadsUnitSize
			_, err := s.stripe.UsageRecords.New(&stripe.UsageRecordParams{
				SubscriptionItem: stripe.String(doc.InstanceReads.ItemID),
				Quantity:         stripe.Int64(units),
				Timestamp:        stripe.Int64(time.Now().Unix()),
				Action:           stripe.String(stripe.UsageRecordActionSet),
			})
			if err != nil {
				return err
			}
		}
		if _, err := s.db.UpdateOne(sctx, bson.M{"_id": req.CustomerId}, bson.M{
			"$set": bson.M{"instance_reads.units": units, "instance_reads.sub_units": subUnits},
		}); err != nil {
			return err
		}
		res = &pb.IncInstanceReadsResponse{
			Period: &pb.UsagePeriod{
				Start: doc.Period.Start,
				End:   doc.Period.End,
			},
			InstanceReads: &pb.AggregateSumUsage{
				Units:     units,
				SubUnits:  subUnits,
				FreeUnits: FreeInstanceReadUnits - units,
			},
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
		"%s period reads: units=%d sub_units=%d free_units=%d",
		req.CustomerId,
		res.InstanceReads.Units,
		res.InstanceReads.SubUnits,
		res.InstanceReads.FreeUnits,
	)
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
		if err := common.StatusCheck(doc.Status); err != nil {
			return err
		}
		var units, subUnits int64
		subUnits = doc.InstanceWrites.SubUnits + req.IncCount
		if subUnits >= InstanceWritesUnitSize {
			addedUnits := subUnits / InstanceWritesUnitSize
			units = addedUnits + doc.InstanceWrites.Units
			if units > FreeInstanceWriteUnits && !doc.Billable {
				return common.ErrExceedsFreeUnits
			}
			subUnits = subUnits % InstanceWritesUnitSize
			_, err := s.stripe.UsageRecords.New(&stripe.UsageRecordParams{
				SubscriptionItem: stripe.String(doc.InstanceWrites.ItemID),
				Quantity:         stripe.Int64(units),
				Timestamp:        stripe.Int64(time.Now().Unix()),
				Action:           stripe.String(stripe.UsageRecordActionSet),
			})
			if err != nil {
				return err
			}
		}
		if _, err := s.db.UpdateOne(sctx, bson.M{"_id": req.CustomerId}, bson.M{
			"$set": bson.M{"instance_writes.units": units, "instance_writes.sub_units": subUnits},
		}); err != nil {
			return err
		}
		res = &pb.IncInstanceWritesResponse{
			Period: &pb.UsagePeriod{
				Start: doc.Period.Start,
				End:   doc.Period.End,
			},
			InstanceWrites: &pb.AggregateSumUsage{
				Units:     units,
				SubUnits:  subUnits,
				FreeUnits: FreeInstanceWriteUnits - units,
			},
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
		"%s period writes: units=%d sub_units=%d free_units=%d",
		req.CustomerId,
		res.InstanceWrites.Units,
		res.InstanceWrites.SubUnits,
		res.InstanceWrites.FreeUnits,
	)
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
