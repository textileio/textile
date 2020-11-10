package service

import (
	"context"
	"crypto/tls"
	"errors"
	"math"
	"net"
	"net/http"
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
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
)

const (
	mib = 1024 * 1024
	gib = 1024 * mib

	Interval      = stripe.PriceRecurringIntervalMonth
	IntervalCount = 1

	StoredDataCostPerInterval      = 0.03 / gib
	StoredDataFreePerInterval      = 5 * gib
	StoredDataUnitSize             = 8 * mib
	StoredDataFreeUnitsPerInterval = StoredDataFreePerInterval / StoredDataUnitSize
	StoredDataUnitCostPerInterval  = StoredDataUnitSize * StoredDataCostPerInterval

	NetworkEgressCostPerInterval      = 0.1 / gib
	NetworkEgressFreePerInterval      = 10 * gib
	NetworkEgressUnitSize             = 8 * mib
	NetworkEgressFreeUnitsPerInterval = NetworkEgressFreePerInterval / NetworkEgressUnitSize
	NetworkEgressUnitCostPerInterval  = NetworkEgressUnitSize * NetworkEgressCostPerInterval

	InstanceReadsCostPerInterval      = 0.1 / 100000
	InstanceReadsFreePerInterval      = 500000
	InstanceReadsUnitSize             = 500
	InstanceReadsFreeUnitsPerInterval = InstanceReadsFreePerInterval / InstanceReadsUnitSize
	InstanceReadsUnitCostPerInterval  = InstanceReadsUnitSize * InstanceReadsCostPerInterval

	InstanceWritesCostPerInterval      = 0.2 / 100000
	InstanceWritesFreePerInterval      = 200000
	InstanceWritesUnitSize             = 500
	InstanceWritesFreeUnitsPerInterval = InstanceWritesFreePerInterval / InstanceWritesUnitSize
	InstanceWritesUnitCostPerInterval  = InstanceWritesUnitSize * InstanceWritesCostPerInterval
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

func NewService(ctx context.Context, config Config) (*Service, error) {
	if config.Debug {
		if err := util.SetLogLevels(map[string]logging.LogLevel{
			"billing": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}

	sc, err := newStripeClient(config.StripeAPIURL, config.StripeAPIKey)
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
			Keys:    bson.D{{"customer_id", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{"parent_key", 1}},
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

	if s.config.StoredDataPriceID == "" {
		s.config.StoredDataPriceID, err = s.createPrice(
			sc,
			"Stored data",
			stripe.PriceRecurringAggregateUsageLastEver,
			StoredDataFreeUnitsPerInterval,
			StoredDataUnitCostPerInterval,
		)
		if err != nil {
			return nil, err
		}
	}
	if s.config.NetworkEgressPriceID == "" {
		s.config.NetworkEgressPriceID, err = s.createPrice(
			sc,
			"Network egress",
			stripe.PriceRecurringAggregateUsageSum,
			NetworkEgressFreeUnitsPerInterval,
			NetworkEgressUnitCostPerInterval,
		)
		if err != nil {
			return nil, err
		}
	}
	if s.config.InstanceReadsPriceID == "" {
		s.config.InstanceReadsPriceID, err = s.createPrice(
			sc,
			"ThreadDB reads",
			stripe.PriceRecurringAggregateUsageSum,
			InstanceReadsFreeUnitsPerInterval,
			InstanceReadsUnitCostPerInterval,
		)
		if err != nil {
			return nil, err
		}
	}
	if s.config.InstanceWritesPriceID == "" {
		s.config.InstanceWritesPriceID, err = s.createPrice(
			sc,
			"ThreadDB writes",
			stripe.PriceRecurringAggregateUsageSum,
			InstanceWritesFreeUnitsPerInterval,
			InstanceWritesUnitCostPerInterval,
		)
		if err != nil {
			return nil, err
		}
	}
	return s, nil
}

func newStripeClient(url, key string) (*stripec.API, error) {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	if err := http2.ConfigureTransport(transport); err != nil {
		return nil, err
	}
	client := &stripec.API{}
	client.Init(key, &stripe.Backends{
		API: stripe.GetBackendWithConfig(
			stripe.APIBackend,
			&stripe.BackendConfig{
				URL: stripe.String(url),
				HTTPClient: &http.Client{
					Transport: transport,
				},
				LeveledLogger: stripe.DefaultLeveledLogger,
			},
		),
	})
	return client, nil
}

func (s *Service) createPrice(
	client *stripec.API,
	productName string,
	priceAggregateUsage stripe.PriceRecurringAggregateUsage,
	freeUnitsPerInterval int64,
	unitCost float64,
) (string, error) {
	product, err := client.Products.New(&stripe.ProductParams{
		Name: stripe.String(productName),
	})
	if err != nil {
		return "", err
	}
	price, err := client.Prices.New(&stripe.PriceParams{
		Currency: stripe.String(string(stripe.CurrencyUSD)),
		Product:  stripe.String(product.ID),
		Recurring: &stripe.PriceRecurringParams{
			AggregateUsage: stripe.String(string(priceAggregateUsage)),
			Interval:       stripe.String(string(Interval)),
			IntervalCount:  stripe.Int64(IntervalCount),
			UsageType:      stripe.String(string(stripe.PriceRecurringUsageTypeMetered)),
		},
		Tiers: []*stripe.PriceTierParams{
			{
				UpTo:       stripe.Int64(freeUnitsPerInterval),
				UnitAmount: stripe.Int64(0),
			},
			{
				UpToInf:           stripe.Bool(true),
				UnitAmountDecimal: stripe.Float64(unitCost),
			},
		},
		TiersMode:     stripe.String(string(stripe.PriceTiersModeGraduated)),
		BillingScheme: stripe.String(string(stripe.PriceBillingSchemeTiered)),
	})
	if err != nil {
		return "", err
	}
	return price.ID, nil
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
	Key        string `bson:"_id"`
	CustomerID string `bson:"customer_id"`
	ParentKey  string `bson:"parent_key"`
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
	if req.ParentKey != "" {
		if r := s.db.FindOne(ctx, bson.M{"_id": req.ParentKey}); r.Err() != nil {
			return nil, r.Err()
		}
	}
	doc := &Customer{
		Key:        req.Key,
		CustomerID: customer.ID,
		ParentKey:  req.ParentKey,
		Email:      customer.Email,
		CreatedAt:  time.Now().Unix(),
	}
	if err := s.createSubscription(doc); err != nil {
		return nil, err
	}
	if _, err := s.db.InsertOne(ctx, doc); err != nil {
		return nil, err
	}
	log.Debugf("created customer %s with id %s", doc.Key, doc.CustomerID)
	return &pb.CreateCustomerResponse{
		CustomerId: doc.CustomerID,
	}, nil
}

func (s *Service) createSubscription(cus *Customer) error {
	sub, err := s.stripe.Subscriptions.New(&stripe.SubscriptionParams{
		Customer: stripe.String(cus.CustomerID),
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
	lck := s.semaphores.Get(customerLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOne(ctx, bson.M{"_id": req.Key})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	if err := r.Decode(&doc); err != nil {
		return nil, err
	}
	log.Debugf("got customer %s", doc.Key)
	return &pb.GetCustomerResponse{
		CustomerId:     doc.CustomerID,
		ParentKey:      doc.ParentKey,
		Email:          doc.Email,
		Status:         doc.Status,
		Balance:        doc.Balance,
		Billable:       doc.Billable,
		Delinquent:     doc.Delinquent,
		CreatedAt:      doc.CreatedAt,
		Period:         periodToPb(doc.Period),
		StoredData:     usageToPb(doc.StoredData, StoredDataUnitSize, StoredDataFreeUnitsPerInterval),
		NetworkEgress:  usageToPb(doc.NetworkEgress, NetworkEgressUnitSize, NetworkEgressFreeUnitsPerInterval),
		InstanceReads:  usageToPb(doc.InstanceReads, InstanceReadsUnitSize, InstanceReadsFreeUnitsPerInterval),
		InstanceWrites: usageToPb(doc.InstanceWrites, InstanceWritesUnitSize, InstanceWritesFreeUnitsPerInterval),
	}, nil
}

func periodToPb(period Period) *pb.Period {
	return &pb.Period{
		Start: period.Start,
		End:   period.End,
	}
}

func usageToPb(usage Usage, unitSize, freeUnits int64) *pb.Usage {
	free := (freeUnits * unitSize) - usage.Total
	if free < 0 {
		free = 0
	}
	return &pb.Usage{
		Units: usage.Units,
		Total: usage.Total,
		Free:  free,
	}
}

func (s *Service) GetCustomerSession(ctx context.Context, req *pb.GetCustomerSessionRequest) (
	*pb.GetCustomerSessionResponse, error) {
	lck := s.semaphores.Get(customerLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOne(ctx, bson.M{"_id": req.Key})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	if err := r.Decode(&doc); err != nil {
		return nil, err
	}
	session, err := s.stripe.BillingPortalSessions.New(&stripe.BillingPortalSessionParams{
		Customer:  stripe.String(doc.CustomerID),
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
	doc, err := s.getCustomer(ctx, "customer_id", req.CustomerId)
	if err != nil {
		return nil, err
	}
	lck := s.semaphores.Get(customerLock(doc.Key))
	lck.Acquire()
	defer lck.Release()

	if _, err := s.db.UpdateOne(ctx, bson.M{"_id": doc.Key}, bson.M{
		"$set": bson.M{"balance": req.Balance, "billable": req.Billable, "delinquent": req.Delinquent},
	}); err != nil {
		return nil, err
	}
	log.Debugf("updated customer %s", doc.Key)
	return &pb.UpdateCustomerResponse{}, nil
}

func (s *Service) UpdateCustomerSubscription(ctx context.Context, req *pb.UpdateCustomerSubscriptionRequest) (
	*pb.UpdateCustomerSubscriptionResponse, error) {
	doc, err := s.getCustomer(ctx, "customer_id", req.CustomerId)
	if err != nil {
		return nil, err
	}
	lck := s.semaphores.Get(customerLock(doc.Key))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOneAndUpdate(ctx, bson.M{"_id": doc.Key}, bson.M{
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
		if _, err := s.db.UpdateOne(ctx, bson.M{"_id": pre.Key}, bson.M{
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

	log.Debugf("updated subscription with status '%s' for %s", req.Status, pre.Key)
	return &pb.UpdateCustomerSubscriptionResponse{}, nil
}

func (s *Service) RecreateCustomerSubscription(ctx context.Context, req *pb.RecreateCustomerSubscriptionRequest) (
	*pb.RecreateCustomerSubscriptionResponse, error) {
	lck := s.semaphores.Get(customerLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOne(ctx, bson.M{"_id": req.Key})
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
	if _, err := s.db.UpdateOne(ctx, bson.M{"_id": req.Key}, bson.M{
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

	log.Debugf("recreated subscription for %s", req.Key)
	return &pb.RecreateCustomerSubscriptionResponse{}, nil
}

func (s *Service) DeleteCustomer(ctx context.Context, req *pb.DeleteCustomerRequest) (
	*pb.DeleteCustomerResponse, error) {
	lck := s.semaphores.Get(customerLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOne(ctx, bson.M{"_id": req.Key})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	if err := r.Decode(&doc); err != nil {
		return nil, err
	}
	if _, err := s.stripe.Customers.Del(doc.CustomerID, nil); err != nil {
		return nil, err
	}
	if _, err := s.db.DeleteOne(ctx, bson.M{"_id": req.Key}); err != nil {
		return nil, err
	}
	log.Debugf("deleted customer %s", req.Key)
	return &pb.DeleteCustomerResponse{}, nil
}

func (s *Service) IncStoredData(ctx context.Context, req *pb.IncStoredDataRequest) (*pb.IncStoredDataResponse, error) {
	lck := s.semaphores.Get(customerLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOne(ctx, bson.M{"_id": req.Key})
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
		doc.Key,
		doc.StoredData,
		"stored_data",
		req.IncSize,
		StoredDataUnitSize,
		StoredDataFreeUnitsPerInterval,
		doc.Billable,
	)
	if err != nil {
		return nil, err
	}

	log.Debugf(
		"%s period data: units=%d total=%d free=%d",
		req.Key,
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
	key string,
	usage Usage,
	usageKey string,
	inc, unitSize, freeUnits int64,
	billable bool,
) (*pb.Usage, error) {
	total := usage.Total + inc
	if total < 0 {
		total = 0
	}
	update := bson.M{usageKey + ".total": total}
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
		update[usageKey+".units"] = units
	}
	if _, err := s.db.UpdateOne(ctx, bson.M{"_id": key}, bson.M{"$set": update}); err != nil {
		return nil, err
	}
	free := (freeUnits * unitSize) - total
	if free < 0 {
		free = 0
	}
	return &pb.Usage{
		Units: units,
		Total: total,
		Free:  free,
	}, nil
}

func (s *Service) IncNetworkEgress(ctx context.Context, req *pb.IncNetworkEgressRequest) (
	*pb.IncNetworkEgressResponse, error) {
	lck := s.semaphores.Get(customerLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOne(ctx, bson.M{"_id": req.Key})
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
		doc.Key,
		doc.NetworkEgress,
		"network_egress",
		req.IncSize,
		NetworkEgressUnitSize,
		NetworkEgressFreeUnitsPerInterval,
		doc.Billable,
	)
	if err != nil {
		return nil, err
	}

	log.Debugf("%s period egress: units=%d total=%d free=%d",
		req.Key,
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
	lck := s.semaphores.Get(customerLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOne(ctx, bson.M{"_id": req.Key})
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
		doc.Key,
		doc.InstanceReads,
		"instance_reads",
		req.IncCount,
		InstanceReadsUnitSize,
		InstanceReadsFreeUnitsPerInterval,
		doc.Billable,
	)
	if err != nil {
		return nil, err
	}

	log.Debugf(
		"%s period reads: units=%d total=%d free=%d",
		req.Key,
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
	lck := s.semaphores.Get(customerLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	r := s.db.FindOne(ctx, bson.M{"_id": req.Key})
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
		doc.Key,
		doc.InstanceWrites,
		"instance_writes",
		req.IncCount,
		InstanceWritesUnitSize,
		InstanceWritesFreeUnitsPerInterval,
		doc.Billable,
	)
	if err != nil {
		return nil, err
	}

	log.Debugf(
		"%s period writes: units=%d total=%d free=%d",
		req.Key,
		usage.Units,
		usage.Total,
		usage.Free,
	)
	return &pb.IncInstanceWritesResponse{
		Period:         periodToPb(doc.Period),
		InstanceWrites: usage,
	}, nil
}

func (s *Service) getCustomer(ctx context.Context, key, val string) (*Customer, error) {
	r := s.db.FindOne(ctx, bson.M{key: val})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	return &doc, r.Decode(&doc)
}
