package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"time"

	logging "github.com/ipfs/go-log"
	ma "github.com/multiformats/go-multiaddr"
	cron "github.com/robfig/cron/v3"
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
	defaultPageSize = 25
	maxPageSize     = 1000

	reporterTimeout = time.Hour

	mib = 1024 * 1024
	gib = 1024 * mib

	Interval      = stripe.PriceRecurringIntervalMonth
	IntervalCount = 1

	StoredDataCostPerInterval     = 0.03 / gib
	StoredDataFreePerInterval     = 5 * gib
	StoredDataUnitSize            = 8 * mib
	StoredDataUnitCostPerInterval = StoredDataUnitSize * StoredDataCostPerInterval

	NetworkEgressCostPerInterval     = 0.1 / gib
	NetworkEgressFreePerInterval     = 10 * gib
	NetworkEgressUnitSize            = 8 * mib
	NetworkEgressUnitCostPerInterval = NetworkEgressUnitSize * NetworkEgressCostPerInterval

	InstanceReadsCostPerInterval     = 0.1 / 100000
	InstanceReadsFreePerInterval     = 500000
	InstanceReadsUnitSize            = 500
	InstanceReadsUnitCostPerInterval = InstanceReadsUnitSize * InstanceReadsCostPerInterval

	InstanceWritesCostPerInterval     = 0.2 / 100000
	InstanceWritesFreePerInterval     = 200000
	InstanceWritesUnitSize            = 500
	InstanceWritesUnitCostPerInterval = InstanceWritesUnitSize * InstanceWritesCostPerInterval
)

var log = logging.Logger("billing")

type Service struct {
	config     Config
	server     *grpc.Server
	stripe     *stripec.API
	db         *mongo.Collection
	gateway    *gateway.Gateway
	reporter   *cron.Cron
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

	StoredDataFreePriceID     string
	NetworkEgressFreePriceID  string
	InstanceReadsFreePriceID  string
	InstanceWritesFreePriceID string

	StoredDataPaidPriceID     string
	NetworkEgressPaidPriceID  string
	InstanceReadsPaidPriceID  string
	InstanceWritesPaidPriceID string

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
			Keys: bson.D{{"parent_key", 1}, {"created_at", 1}},
		},
	})
	if err != nil {
		return nil, err
	}
	for _, index := range indexes {
		log.Infof("created index: %s", index)
	}

	s := &Service{
		config:   config,
		db:       customers,
		stripe:   sc,
		reporter: cron.New(),
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

	if _, err := s.reporter.AddFunc("@daily", func() {
		if err := s.reportUsage(); err != nil {
			log.Errorf("reporting usage: %v", err)
		}
	}); err != nil {
		return nil, err
	}

	if s.config.StoredDataFreePriceID == "" {
		s.config.StoredDataFreePriceID, err = s.createPrice(
			sc,
			"Stored data",
			StoredDataUnitCostPerInterval,
		)
		if err != nil {
			return nil, err
		}
	}
	if s.config.NetworkEgressFreePriceID == "" {
		s.config.NetworkEgressFreePriceID, err = s.createPrice(
			sc,
			"Network egress",
			NetworkEgressUnitCostPerInterval,
		)
		if err != nil {
			return nil, err
		}
	}
	if s.config.InstanceReadsFreePriceID == "" {
		s.config.InstanceReadsFreePriceID, err = s.createPrice(
			sc,
			"ThreadDB reads",
			InstanceReadsUnitCostPerInterval,
		)
		if err != nil {
			return nil, err
		}
	}
	if s.config.InstanceWritesFreePriceID == "" {
		s.config.InstanceWritesFreePriceID, err = s.createPrice(
			sc,
			"ThreadDB writes",
			InstanceWritesUnitCostPerInterval,
		)
		if err != nil {
			return nil, err
		}
	}

	if s.config.StoredDataPaidPriceID == "" {
		s.config.StoredDataPaidPriceID, err = s.createPrice(
			sc,
			"Stored data (free quota)",
			0,
		)
		if err != nil {
			return nil, err
		}
	}
	if s.config.NetworkEgressPaidPriceID == "" {
		s.config.NetworkEgressPaidPriceID, err = s.createPrice(
			sc,
			"Network egress (free quota)",
			0,
		)
		if err != nil {
			return nil, err
		}
	}
	if s.config.InstanceReadsPaidPriceID == "" {
		s.config.InstanceReadsPaidPriceID, err = s.createPrice(
			sc,
			"ThreadDB reads (free quota)",
			0,
		)
		if err != nil {
			return nil, err
		}
	}
	if s.config.InstanceWritesPaidPriceID == "" {
		s.config.InstanceWritesPaidPriceID, err = s.createPrice(
			sc,
			"ThreadDB writes (free quota)",
			0,
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

func (s *Service) createPrice(client *stripec.API, productName string, unitCost float64) (string, error) {
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
			AggregateUsage: stripe.String(string(stripe.PriceRecurringAggregateUsageSum)),
			Interval:       stripe.String(string(Interval)),
			IntervalCount:  stripe.Int64(IntervalCount),
			UsageType:      stripe.String(string(stripe.PriceRecurringUsageTypeMetered)),
		},
		Tiers: []*stripe.PriceTierParams{
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
	s.reporter.Start()
	return nil
}

func (s *Service) Stop(force bool) error {
	if force {
		s.server.Stop()
	} else {
		s.server.GracefulStop()
	}
	s.reporter.Stop()
	s.semaphores.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := s.gateway.Stop(); err != nil {
		return err
	}
	return s.db.Database().Client().Disconnect(ctx)
}

type Customer struct {
	Key                string `bson:"_id"`
	CustomerID         string `bson:"customer_id"`
	ParentKey          string `bson:"parent_key"`
	Email              string `bson:"email"`
	SubscriptionStatus string `bson:"subscription_status"`
	Balance            int64  `bson:"balance"`
	Billable           bool   `bson:"billable"`
	Delinquent         bool   `bson:"delinquent"`
	CreatedAt          int64  `bson:"created_at"`

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
	FreeItemID string `bson:"free_item_id"`
	PaidItemID string `bson:"paid_item_id"`
	Total      int64  `bson:"total"`
}

func (c *Customer) AccountStatus() string {
	if c.ParentKey != "" {
		return "dependent"
	} else if c.Delinquent {
		return "delinquent"
	} else if c.Billable {
		return "pay-as-you-go"
	} else {
		return "free-quota-only"
	}
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
		if _, err := s.getCustomer(ctx, "_id", req.ParentKey); err != nil {
			return nil, err
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
			{Price: stripe.String(s.config.StoredDataFreePriceID)},
			{Price: stripe.String(s.config.StoredDataPaidPriceID)},
			{Price: stripe.String(s.config.NetworkEgressFreePriceID)},
			{Price: stripe.String(s.config.NetworkEgressPaidPriceID)},
			{Price: stripe.String(s.config.InstanceReadsFreePriceID)},
			{Price: stripe.String(s.config.InstanceReadsPaidPriceID)},
			{Price: stripe.String(s.config.InstanceWritesFreePriceID)},
			{Price: stripe.String(s.config.InstanceWritesPaidPriceID)},
		},
	})
	if err != nil {
		return err
	}
	cus.SubscriptionStatus = string(sub.Status)
	cus.Period = Period{
		Start: sub.CurrentPeriodStart,
		End:   sub.CurrentPeriodEnd,
	}
	for _, item := range sub.Items.Data {
		switch item.Price.ID {
		case s.config.StoredDataFreePriceID:
			cus.StoredData.PaidItemID = item.ID
		case s.config.StoredDataPaidPriceID:
			cus.StoredData.FreeItemID = item.ID
		case s.config.NetworkEgressFreePriceID:
			cus.NetworkEgress.PaidItemID = item.ID
		case s.config.NetworkEgressPaidPriceID:
			cus.NetworkEgress.FreeItemID = item.ID
		case s.config.InstanceReadsFreePriceID:
			cus.InstanceReads.PaidItemID = item.ID
		case s.config.InstanceReadsPaidPriceID:
			cus.InstanceReads.FreeItemID = item.ID
		case s.config.InstanceWritesFreePriceID:
			cus.InstanceWrites.PaidItemID = item.ID
		case s.config.InstanceWritesPaidPriceID:
			cus.InstanceWrites.FreeItemID = item.ID
		}
	}
	return nil
}

func (s *Service) GetCustomer(ctx context.Context, req *pb.GetCustomerRequest) (
	*pb.GetCustomerResponse, error) {
	lck := s.semaphores.Get(customerLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	doc, err := s.getCustomer(ctx, "_id", req.Key)
	if err != nil {
		return nil, err
	}
	log.Debugf("got customer %s", doc.Key)
	return s.customerToPb(ctx, doc)
}

func periodToPb(period Period) *pb.Period {
	return &pb.Period{
		Start: period.Start,
		End:   period.End,
	}
}

func usageToPb(usage Usage, freeSize int64) *pb.Usage {
	free := freeSize - usage.Total
	if free < 0 {
		free = 0
	}
	return &pb.Usage{
		Total: usage.Total,
		Free:  free,
	}
}

func (s *Service) customerToPb(ctx context.Context, doc *Customer) (*pb.GetCustomerResponse, error) {
	deps, err := s.db.CountDocuments(ctx, bson.M{"parent_id": doc.Key})
	if err != nil {
		return nil, err
	}
	return &pb.GetCustomerResponse{
		Key:                doc.Key,
		CustomerId:         doc.CustomerID,
		ParentKey:          doc.ParentKey,
		Email:              doc.Email,
		AccountStatus:      doc.AccountStatus(),
		SubscriptionStatus: doc.SubscriptionStatus,
		Balance:            doc.Balance,
		Billable:           doc.Billable,
		Delinquent:         doc.Delinquent,
		CreatedAt:          doc.CreatedAt,
		Period:             periodToPb(doc.Period),
		StoredData:         usageToPb(doc.StoredData, StoredDataFreePerInterval),
		NetworkEgress:      usageToPb(doc.NetworkEgress, NetworkEgressFreePerInterval),
		InstanceReads:      usageToPb(doc.InstanceReads, InstanceReadsFreePerInterval),
		InstanceWrites:     usageToPb(doc.InstanceWrites, InstanceWritesFreePerInterval),
		Dependents:         deps,
	}, nil
}

func (s *Service) GetCustomerSession(ctx context.Context, req *pb.GetCustomerSessionRequest) (
	*pb.GetCustomerSessionResponse, error) {
	lck := s.semaphores.Get(customerLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	doc, err := s.getCustomer(ctx, "_id", req.Key)
	if err != nil {
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

func (s *Service) ListDependentCustomers(ctx context.Context, req *pb.ListDependentCustomersRequest) (
	*pb.ListDependentCustomersResponse, error) {
	lck := s.semaphores.Get(customerLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	doc, err := s.getCustomer(ctx, "_id", req.Key)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"parent_key": doc.Key}
	if req.Offset > 0 {
		filter["created_at"] = bson.M{"$gt": req.Offset}
	}
	opts := &options.FindOptions{}
	if req.Limit > 0 {
		if req.Limit > maxPageSize {
			return nil, fmt.Errorf("maximum limit is %d", maxPageSize)
		}
		opts.SetLimit(req.Limit)
	} else {
		opts.SetLimit(defaultPageSize)
	}
	cursor, err := s.db.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var list []*pb.GetCustomerResponse
	for cursor.Next(ctx) {
		var doc Customer
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		cus, err := s.customerToPb(ctx, &doc)
		if err != nil {
			return nil, err
		}
		list = append(list, cus)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	var next int64
	if len(list) > 0 {
		next = list[len(list)-1].CreatedAt
	}
	log.Debugf("listed %d customers", len(list))
	return &pb.ListDependentCustomersResponse{
		Customers:  list,
		NextOffset: next,
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
	if _, err := s.db.UpdateOne(ctx, bson.M{"_id": doc.Key}, bson.M{
		"$set": bson.M{
			"subscription_status": req.Status,
			"period.start":        req.Period.Start,
			"period.end":          req.Period.End,
		},
	}); err != nil {
		return nil, err
	}
	log.Debugf("updated subscription with status '%s' for %s", req.Status, doc.Key)
	return &pb.UpdateCustomerSubscriptionResponse{}, nil
}

func (s *Service) RecreateCustomerSubscription(ctx context.Context, req *pb.RecreateCustomerSubscriptionRequest) (
	*pb.RecreateCustomerSubscriptionResponse, error) {
	lck := s.semaphores.Get(customerLock(req.Key))
	lck.Acquire()
	defer lck.Release()

	doc, err := s.getCustomer(ctx, "_id", req.Key)
	if err != nil {
		return nil, err
	}
	if err := common.StatusCheck(doc.SubscriptionStatus); err == nil {
		return nil, common.ErrSubscriptionExists
	} else if !errors.Is(err, common.ErrSubscriptionCanceled) {
		return nil, err
	}
	if err := s.createSubscription(doc); err != nil {
		return nil, err
	}
	if _, err := s.db.UpdateOne(ctx, bson.M{"_id": req.Key}, bson.M{
		"$set": bson.M{
			"subscription_status": doc.SubscriptionStatus,
			"period":              doc.Period,
			"stored_data":         doc.StoredData,
			"network_egress":      doc.NetworkEgress,
			"instance_reads":      doc.InstanceReads,
			"instance_writes":     doc.InstanceWrites,
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

	doc, err := s.getCustomer(ctx, "_id", req.Key)
	if err != nil {
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

func (s *Service) IncCustomerUsage(ctx context.Context, req *pb.IncCustomerUsageRequest) (*pb.IncCustomerUsageResponse, error) {
	return s.handleCustomerUsage(ctx, req.Key, req)
}

func (s *Service) handleCustomerUsage(
	ctx context.Context,
	key string,
	req *pb.IncCustomerUsageRequest,
) (*pb.IncCustomerUsageResponse, error) {
	lck := s.semaphores.Get(customerLock(key))
	lck.Acquire()
	defer lck.Release()

	cus, err := s.getCustomer(ctx, "_id", key)
	if err != nil {
		return nil, err
	}
	if cus.ParentKey != "" {
		if _, err := s.handleCustomerUsage(ctx, cus.ParentKey, req); err != nil {
			return nil, err
		}
	}
	if err := common.StatusCheck(cus.SubscriptionStatus); err != nil {
		return nil, err
	}
	sd, err := s.handleUsage(
		ctx,
		cus.Key,
		cus.StoredData,
		"stored_data",
		req.StoredDataIncSize,
		StoredDataFreePerInterval,
		cus.Billable,
	)
	if err != nil {
		return nil, err
	}
	log.Debugf(
		"%s period data: total=%d free=%d",
		cus.Key,
		sd.Total,
		sd.Free,
	)
	ne, err := s.handleUsage(
		ctx,
		cus.Key,
		cus.NetworkEgress,
		"network_egress",
		req.NetworkEgressIncSize,
		NetworkEgressFreePerInterval,
		cus.Billable,
	)
	if err != nil {
		return nil, err
	}
	log.Debugf("%s period egress: total=%d free=%d",
		cus.Key,
		ne.Total,
		ne.Free,
	)
	ir, err := s.handleUsage(
		ctx,
		cus.Key,
		cus.InstanceReads,
		"instance_reads",
		req.InstanceReadsIncCount,
		InstanceReadsFreePerInterval,
		cus.Billable,
	)
	if err != nil {
		return nil, err
	}
	log.Debugf(
		"%s period reads: total=%d free=%d",
		cus.Key,
		ir.Total,
		ir.Free,
	)
	iw, err := s.handleUsage(
		ctx,
		cus.Key,
		cus.InstanceWrites,
		"instance_writes",
		req.InstanceWritesIncCount,
		InstanceWritesFreePerInterval,
		cus.Billable,
	)
	if err != nil {
		return nil, err
	}
	log.Debugf(
		"%s period writes: total=%d free=%d",
		cus.Key,
		iw.Total,
		iw.Free,
	)
	return &pb.IncCustomerUsageResponse{
		Period:         periodToPb(cus.Period),
		StoredData:     sd,
		NetworkEgress:  ne,
		InstanceReads:  ir,
		InstanceWrites: iw,
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

func (s *Service) handleUsage(
	ctx context.Context,
	key string,
	usage Usage,
	usageKey string,
	inc, freeSize int64,
	billable bool,
) (*pb.Usage, error) {
	total := usage.Total + inc
	if total < 0 {
		total = 0
	}
	if total > freeSize && !billable {
		return nil, common.ErrExceedsFreeQuota
	}
	update := bson.M{usageKey + ".total": total}
	if _, err := s.db.UpdateOne(ctx, bson.M{"_id": key}, bson.M{"$set": update}); err != nil {
		return nil, err
	}
	free := freeSize - total
	if free < 0 {
		free = 0
	}
	return &pb.Usage{
		Total: total,
		Free:  free,
	}, nil
}

func (s *Service) reportUsage() error {
	periodDays := getPeriodDays()

	ctx, cancel := context.WithTimeout(context.Background(), reporterTimeout)
	defer cancel()
	cursor, err := s.db.Find(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("finding customers: %v", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc Customer
		if err := cursor.Decode(&doc); err != nil {
			return fmt.Errorf("decoding customer: %v", err)
		}

		if err := s.reportUnits(
			doc.StoredData,
			StoredDataFreePerInterval,
			StoredDataUnitSize,
			periodDays,
		); err != nil {
			return err
		}
		if err := s.reportUnits(
			doc.NetworkEgress,
			NetworkEgressFreePerInterval,
			NetworkEgressUnitSize,
			periodDays,
		); err != nil {
			return err
		}
		if err := s.reportUnits(
			doc.InstanceReads,
			InstanceReadsFreePerInterval,
			InstanceReadsUnitSize,
			periodDays,
		); err != nil {
			return err
		}
		if err := s.reportUnits(
			doc.InstanceWrites,
			InstanceWritesFreePerInterval,
			InstanceWritesUnitSize,
			periodDays,
		); err != nil {
			return err
		}

		// Reset all usage totals.
		if _, err := s.db.UpdateOne(ctx, bson.M{"_id": doc.Key}, bson.M{
			"$set": bson.M{
				"stored_data.total":     0,
				"network_egress.total":  0,
				"instance_reads.total":  0,
				"instance_writes.total": 0,
			},
		}); err != nil {
			return err
		}
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("cursor error: %v", err)
	}
	return nil
}

func (s *Service) reportUnits(usage Usage, productFreeSize, productUnitSize, periodDays int64) error {
	var freeSize, paidSize int64
	if usage.Total > productFreeSize {
		freeSize = productFreeSize
		paidSize = usage.Total - productFreeSize
	} else {
		freeSize = usage.Total
	}
	freeUnits := int64(math.Round(float64(freeSize) / float64(productUnitSize) / float64(periodDays)))
	if freeUnits > 0 {
		if _, err := s.stripe.UsageRecords.New(&stripe.UsageRecordParams{
			SubscriptionItem: stripe.String(usage.FreeItemID),
			Quantity:         stripe.Int64(freeUnits),
			Timestamp:        stripe.Int64(time.Now().Unix()),
			Action:           stripe.String(stripe.UsageRecordActionSet),
		}); err != nil {
			return err
		}
	}
	paidUnits := int64(math.Round(float64(paidSize) / float64(productUnitSize) / float64(periodDays)))
	if paidUnits > 0 {
		if _, err := s.stripe.UsageRecords.New(&stripe.UsageRecordParams{
			SubscriptionItem: stripe.String(usage.PaidItemID),
			Quantity:         stripe.Int64(paidUnits),
			Timestamp:        stripe.Int64(time.Now().Unix()),
			Action:           stripe.String(stripe.UsageRecordActionIncrement),
		}); err != nil {
			return err
		}
	}
	return nil
}

func getPeriodDays() int64 {
	// Go to next month
	d1 := time.Now().AddDate(0, 1, 0)
	// Zeroing days will cause time to normalize the date to the last day of the current month.
	d2 := time.Date(d1.Year(), d1.Month(), 0, 0, 0, 0, 0, time.UTC)
	return int64(d2.Day())
}
