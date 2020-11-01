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
	nutil "github.com/textileio/go-threads/net/util"
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

	StoredDataFreeUnits     = 100                               // 5 Gib
	NetworkEgressFreeUnits  = 500 * mib / NetworkEgressUnitSize // 500 Mib per day
	InstanceReadsFreeUnits  = 1                                 // 10,000 per day
	InstanceWritesFreeUnits = 1                                 // 5,000 per day
)

var log = logging.Logger("billing")

type Service struct {
	config     Config
	server     *grpc.Server
	stripe     *stripec.API
	db         *mongo.Collection
	gateway    *gateway.Gateway
	semaphores *nutil.SemaphorePool
}

var _ pb.APIServiceServer = (*Service)(nil)

var _ nutil.SemaphoreKey = (*customerLock)(nil)

type customerLock string

func (l customerLock) Key() string {
	return string(l)
}

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
	s.semaphores = nutil.NewSemaphorePool(1)
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
	s.semaphores.Stop()
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

	Period         Period `bson:"period"`
	StoredData     Usage  `bson:"stored_data"`
	NetworkEgress  Usage  `bson:"network_egress"`
	InstanceReads  Usage  `bson:"instance_reads"`
	InstanceWrites Usage  `bson:"instance_writes"`
}

type Period struct {
	Start int64 `bson:"start"`
	End   int64 `bson:"end"`
}

type Usage struct {
	ItemID string `bson:"item_id"`
	Units  int64  `bson:"units"`
	Total  int64  `bson:"total"`
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
	cus.Period = Period{
		Start: sub.CurrentPeriodStart,
		End:   sub.CurrentPeriodEnd,
	}
	for _, item := range sub.Items.Data {
		switch item.Price.ID {
		case s.config.StoredDataPriceID:
			cus.StoredData.ItemID = item.ID // Retain existing units since this is "last ever" usage
		case s.config.NetworkEgressPriceID:
			cus.NetworkEgress = Usage{ItemID: item.ID}
		case s.config.InstanceReadsPriceID:
			cus.InstanceReads = Usage{ItemID: item.ID}
		case s.config.InstanceWritesPriceID:
			cus.InstanceWrites = Usage{ItemID: item.ID}
		}
	}
	return nil
}

func (s *Service) GetCustomer(ctx context.Context, req *pb.GetCustomerRequest) (
	*pb.GetCustomerResponse, error) {
	lck := s.semaphores.Get(customerLock(req.CustomerId))
	lck.Acquire()
	defer lck.Release()

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
		Email:          doc.Email,
		Status:         doc.Status,
		Balance:        doc.Balance,
		Billable:       doc.Billable,
		Delinquent:     doc.Delinquent,
		CreatedAt:      doc.CreatedAt,
		Period:         periodToPb(doc.Period),
		StoredData:     usageToPb(doc.StoredData, StoredDataUnitSize, StoredDataFreeUnits),
		NetworkEgress:  usageToPb(doc.NetworkEgress, NetworkEgressUnitSize, NetworkEgressFreeUnits),
		InstanceReads:  usageToPb(doc.InstanceReads, InstanceReadsUnitSize, InstanceReadsFreeUnits),
		InstanceWrites: usageToPb(doc.InstanceWrites, InstanceWritesUnitSize, InstanceWritesFreeUnits),
	}, nil
}

func periodToPb(period Period) *pb.Period {
	return &pb.Period{
		Start: period.Start,
		End:   period.End,
	}
}

func usageToPb(usage Usage, unitSize, freeUnits int64) *pb.Usage {
	return &pb.Usage{
		Units: usage.Units,
		Total: usage.Total,
		Free:  (freeUnits * unitSize) - usage.Total,
	}
}

func (s *Service) GetCustomerSession(_ context.Context, req *pb.GetCustomerSessionRequest) (
	*pb.GetCustomerSessionResponse, error) {
	lck := s.semaphores.Get(customerLock(req.CustomerId))
	lck.Acquire()
	defer lck.Release()

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
	lck := s.semaphores.Get(customerLock(req.CustomerId))
	lck.Acquire()
	defer lck.Release()

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
	lck := s.semaphores.Get(customerLock(req.CustomerId))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOneAndUpdate(ctx, bson.M{"_id": req.CustomerId}, bson.M{
		"$set": bson.M{
			"status":       req.Status,
			"period.start": req.Period.Start,
			"period.end":   req.Period.End,
		},
	})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var pre Customer
	if err := r.Decode(&pre); err != nil {
		return nil, err
	}
	if pre.Period.End < req.Period.End {
		if _, err := s.db.UpdateOne(ctx, bson.M{"_id": req.CustomerId}, bson.M{
			"$set": bson.M{
				"network_egress.units":      0,
				"network_egress.sub_units":  0,
				"instance_reads.units":      0,
				"instance_reads.sub_units":  0,
				"instance_writes.units":     0,
				"instance_writes.sub_units": 0,
			}}); err != nil {
			return nil, err
		}
	}

	log.Debugf("updated subscription with status '%s' for %s", req.Status, req.CustomerId)
	return &pb.UpdateCustomerSubscriptionResponse{}, nil
}

func (s *Service) RecreateCustomerSubscription(ctx context.Context, req *pb.RecreateCustomerSubscriptionRequest) (
	*pb.RecreateCustomerSubscriptionResponse, error) {
	lck := s.semaphores.Get(customerLock(req.CustomerId))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOne(ctx, bson.M{"_id": req.CustomerId})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	if err := r.Decode(&doc); err != nil {
		return nil, err
	}
	if err := common.StatusCheck(doc.Status); err == nil {
		return nil, common.ErrSubscriptionExists
	} else if !errors.Is(err, common.ErrSubscriptionCanceled) {
		return nil, err
	}
	if err := s.createSubscription(&doc); err != nil {
		return nil, err
	}
	if _, err := s.db.UpdateOne(ctx, bson.M{"_id": req.CustomerId}, bson.M{
		"$set": bson.M{
			"status":          doc.Status,
			"period":          doc.Period,
			"stored_data":     doc.StoredData,
			"network_egress":  doc.NetworkEgress,
			"instance_reads":  doc.InstanceReads,
			"instance_writes": doc.InstanceWrites,
		}}); err != nil {
		return nil, err
	}

	log.Debugf("recreated subscription for %s", req.CustomerId)
	return &pb.RecreateCustomerSubscriptionResponse{}, nil
}

func (s *Service) DeleteCustomer(ctx context.Context, req *pb.DeleteCustomerRequest) (
	*pb.DeleteCustomerResponse, error) {
	lck := s.semaphores.Get(customerLock(req.CustomerId))
	lck.Acquire()
	defer lck.Release()

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
	lck := s.semaphores.Get(customerLock(req.CustomerId))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOne(ctx, bson.M{"_id": req.CustomerId})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	if err := r.Decode(&doc); err != nil {
		return nil, err
	}
	if err := common.StatusCheck(doc.Status); err != nil {
		return nil, err
	}
	usage, err := s.handleUsage(
		ctx,
		doc.ID,
		doc.StoredData,
		"stored_data",
		req.IncSize,
		StoredDataUnitSize,
		StoredDataFreeUnits,
		doc.Billable,
	)
	if err != nil {
		return nil, err
	}

	log.Debugf(
		"%s period data: units=%d total=%d free=%d",
		req.CustomerId,
		usage.Units,
		usage.Total,
		usage.Free,
	)
	return &pb.IncStoredDataResponse{
		Period:     periodToPb(doc.Period),
		StoredData: usage,
	}, nil
}

func (s *Service) handleUsage(
	ctx context.Context,
	customerID string,
	usage Usage,
	key string,
	inc, unitSize, freeUnits int64,
	billable bool,
) (*pb.Usage, error) {
	total := usage.Total + inc
	if total < 0 {
		total = 0
	}
	update := bson.M{key + ".total": total}
	units := int64(math.Round(float64(total) / float64(unitSize)))
	if units > freeUnits && !billable {
		return nil, common.ErrExceedsFreeUnits
	}
	if units != usage.Units {
		if _, err := s.stripe.UsageRecords.New(&stripe.UsageRecordParams{
			SubscriptionItem: stripe.String(usage.ItemID),
			Quantity:         stripe.Int64(units),
			Timestamp:        stripe.Int64(time.Now().Unix()),
			Action:           stripe.String(stripe.UsageRecordActionSet),
		}); err != nil {
			return nil, err
		}
		update[key+".units"] = units
	}
	if _, err := s.db.UpdateOne(ctx, bson.M{"_id": customerID}, bson.M{"$set": update}); err != nil {
		return nil, err
	}
	return &pb.Usage{
		Units: units,
		Total: total,
		Free:  (freeUnits * unitSize) - total,
	}, nil
}

func (s *Service) IncNetworkEgress(ctx context.Context, req *pb.IncNetworkEgressRequest) (
	*pb.IncNetworkEgressResponse, error) {
	lck := s.semaphores.Get(customerLock(req.CustomerId))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOne(ctx, bson.M{"_id": req.CustomerId})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	if err := r.Decode(&doc); err != nil {
		return nil, err
	}
	if err := common.StatusCheck(doc.Status); err != nil {
		return nil, err
	}
	usage, err := s.handleUsage(
		ctx,
		doc.ID,
		doc.NetworkEgress,
		"network_egress",
		req.IncSize,
		NetworkEgressUnitSize,
		NetworkEgressFreeUnits,
		doc.Billable,
	)
	if err != nil {
		return nil, err
	}

	log.Debugf("%s period egress: units=%d total=%d free=%d",
		req.CustomerId,
		usage.Units,
		usage.Total,
		usage.Free,
	)
	res := &pb.IncNetworkEgressResponse{
		Period:        periodToPb(doc.Period),
		NetworkEgress: usage,
	}
	return res, nil
}

func (s *Service) IncInstanceReads(ctx context.Context, req *pb.IncInstanceReadsRequest) (
	*pb.IncInstanceReadsResponse, error) {
	lck := s.semaphores.Get(customerLock(req.CustomerId))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOne(ctx, bson.M{"_id": req.CustomerId})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	if err := r.Decode(&doc); err != nil {
		return nil, err
	}
	if err := common.StatusCheck(doc.Status); err != nil {
		return nil, err
	}
	usage, err := s.handleUsage(
		ctx,
		doc.ID,
		doc.InstanceReads,
		"instance_reads",
		req.IncCount,
		InstanceReadsUnitSize,
		InstanceReadsFreeUnits,
		doc.Billable,
	)
	if err != nil {
		return nil, err
	}

	log.Debugf(
		"%s period reads: units=%d total=%d free=%d",
		req.CustomerId,
		usage.Units,
		usage.Total,
		usage.Free,
	)
	return &pb.IncInstanceReadsResponse{
		Period:        periodToPb(doc.Period),
		InstanceReads: usage,
	}, nil
}

func (s *Service) IncInstanceWrites(ctx context.Context, req *pb.IncInstanceWritesRequest) (
	*pb.IncInstanceWritesResponse, error) {
	lck := s.semaphores.Get(customerLock(req.CustomerId))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOne(ctx, bson.M{"_id": req.CustomerId})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	if err := r.Decode(&doc); err != nil {
		return nil, err
	}
	if err := common.StatusCheck(doc.Status); err != nil {
		return nil, err
	}
	usage, err := s.handleUsage(
		ctx,
		doc.ID,
		doc.InstanceWrites,
		"instance_writes",
		req.IncCount,
		InstanceWritesUnitSize,
		InstanceWritesFreeUnits,
		doc.Billable,
	)
	if err != nil {
		return nil, err
	}

	log.Debugf(
		"%s period writes: units=%d total=%d free=%d",
		req.CustomerId,
		usage.Units,
		usage.Total,
		usage.Free,
	)
	return &pb.IncInstanceWritesResponse{
		Period:         periodToPb(doc.Period),
		InstanceWrites: usage,
	}, nil
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
