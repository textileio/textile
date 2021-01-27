package service

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"math"
	"net"
	"net/http"
	"strings"
	"time"

	logging "github.com/ipfs/go-log/v2"
	ma "github.com/multiformats/go-multiaddr"
	cron "github.com/robfig/cron/v3"
	stripe "github.com/stripe/stripe-go/v72"
	stripec "github.com/stripe/stripe-go/v72/client"
	nutil "github.com/textileio/go-threads/net/util"
	"github.com/textileio/go-threads/util"
	"github.com/textileio/textile/v2/api/billingd/analytics"
	"github.com/textileio/textile/v2/api/billingd/common"
	"github.com/textileio/textile/v2/api/billingd/gateway"
	"github.com/textileio/textile/v2/api/billingd/migrations"
	pb "github.com/textileio/textile/v2/api/billingd/pb"
	mdb "github.com/textileio/textile/v2/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/net/http2"
	"google.golang.org/grpc"
)

const (
	numDaysPerMonth    = 30.4167
	unitPricePrecision = 1e12

	reporterTimeout = time.Hour

	defaultPageSize = 25
	maxPageSize     = 1000

	mib = 1024 * 1024
	gib = 1024 * mib

	duplicateKeyMsg = "E11000 duplicate key error"
)

var (
	log = logging.Logger("billing")

	// ErrCustomerExists indicates a customer already exists.
	ErrCustomerExists = errors.New("customer already exists")
)

type Product struct {
	Key                      string            `bson:"_id"`
	Name                     string            `bson:"name"`
	Price                    float64           `bson:"price"`
	PriceType                PriceType         `bson:"price_type"`
	FreeQuotaSize            int64             `bson:"free_quota_size"`
	FreeQuotaGracePeriodSize int64             `bson:"free_quota_grace_period_size"`
	FreeQuotaInterval        FreeQuotaInterval `bson:"free_quota_interval"`

	Units    string `bson:"units"`
	UnitSize int64  `bson:"unit_size"`

	FreePriceID string `bson:"free_price_id"`
	PaidPriceID string `bson:"paid_price_id"`
}

type PriceType string

const (
	PriceTypeTemporal    PriceType = "temporal"
	PriceTypeIncremental           = "incremental"
)

type FreeQuotaInterval string

const (
	FreeQuotaMonthly FreeQuotaInterval = "monthly"
	FreeQuotaDaily                     = "daily"
)

var Products = []Product{
	{
		Key:                      "stored_data",
		Name:                     "Stored data",
		Price:                    0.03 / gib,
		PriceType:                PriceTypeTemporal,
		FreeQuotaSize:            5 * gib,
		FreeQuotaGracePeriodSize: 1000 * gib,
		FreeQuotaInterval:        FreeQuotaMonthly,
		Units:                    "bytes",
		UnitSize:                 8 * mib,
	},
	{
		Key:                      "network_egress",
		Name:                     "Network egress",
		Price:                    0.1 / gib,
		PriceType:                PriceTypeIncremental,
		FreeQuotaSize:            10 * gib,
		FreeQuotaGracePeriodSize: 1000 * gib,
		FreeQuotaInterval:        FreeQuotaMonthly,
		Units:                    "bytes",
		UnitSize:                 8 * mib,
	},
	{
		Key:                      "instance_reads",
		Name:                     "ThreadDB reads",
		Price:                    0.1 / 100000,
		PriceType:                PriceTypeIncremental,
		FreeQuotaSize:            50000,
		FreeQuotaGracePeriodSize: 1000000,
		FreeQuotaInterval:        FreeQuotaDaily,
		UnitSize:                 100,
	},
	{
		Key:                      "instance_writes",
		Name:                     "ThreadDB writes",
		Price:                    0.2 / 100000,
		PriceType:                PriceTypeIncremental,
		FreeQuotaSize:            20000,
		FreeQuotaGracePeriodSize: 1000000,
		FreeQuotaInterval:        FreeQuotaDaily,
		UnitSize:                 100,
	},
}

type Customer struct {
	Key                string          `bson:"_id"`
	CustomerID         string          `bson:"customer_id"`
	ParentKey          string          `bson:"parent_key"`
	Email              string          `bson:"email"`
	AccountType        mdb.AccountType `bson:"account_type"`
	SubscriptionStatus string          `bson:"subscription_status"`
	Balance            int64           `bson:"balance"`
	Billable           bool            `bson:"billable"`
	Delinquent         bool            `bson:"delinquent"`
	CreatedAt          int64           `bson:"created_at"`
	GracePeriodStart   int64           `bson:"grace_period_start"`

	InvoicePeriod Period `bson:"invoice_period"`

	DailyUsage map[string]Usage `bson:"daily_usage"`
}

type Period struct {
	UnixStart int64 `bson:"unix_start"`
	UnixEnd   int64 `bson:"unix_end"`
}

type Usage struct {
	Total int64 `bson:"total"`

	FreeItemID string `bson:"free_item_id"`
	PaidItemID string `bson:"paid_item_id"`
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

type Service struct {
	config     Config
	analytics  *analytics.Client
	server     *grpc.Server
	stripe     *stripec.API
	gateway    *gateway.Gateway
	reporter   *cron.Cron
	semaphores *nutil.SemaphorePool

	pdb *mongo.Collection
	cdb *mongo.Collection

	products map[string]Product
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

	SegmentAPIKey string
	SegmentPrefix string

	DBURI  string
	DBName string

	GatewayHostAddr ma.Multiaddr

	FreeQuotaGracePeriod time.Duration

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

	// Configure analytics client
	ac, err := analytics.NewClient(config.SegmentAPIKey, config.SegmentPrefix, config.Debug)
	if err != nil {
		return nil, err
	}

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.DBURI))
	if err != nil {
		return nil, err
	}
	db := client.Database(config.DBName)
	if err = migrations.Migrate(db); err != nil {
		return nil, err
	}

	pdb := db.Collection("products")
	cdb := db.Collection("customers")
	indexes, err := cdb.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"customer_id", 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
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
		config:    config,
		stripe:    sc,
		analytics: ac,
		reporter:  cron.New(),
		pdb:       pdb,
		cdb:       cdb,
		products:  make(map[string]Product),
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

	for _, product := range Products {
		var doc Product
		if _, err := pdb.InsertOne(ctx, product); err != nil && strings.Contains(err.Error(), duplicateKeyMsg) {
			r := pdb.FindOne(ctx, bson.M{"_id": product.Key})
			if r.Err() != nil {
				return nil, r.Err()
			}
			if err := r.Decode(&doc); err != nil {
				return nil, err
			}
		} else if err == nil {
			doc = product
			doc.FreePriceID, err = s.createPrice(sc, doc.Name+" (free quota)", 0)
			if err != nil {
				return nil, err
			}
			up := getUnitPrice(doc)
			doc.PaidPriceID, err = s.createPrice(sc, doc.Name, up)
			if err != nil {
				return nil, err
			}
			if _, err = pdb.UpdateOne(ctx, bson.M{"_id": doc.Key}, bson.M{"$set": doc}); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
		s.products[product.Key] = doc
	}
	return s, nil
}

func getUnitPrice(product Product) float64 {
	var unitPrice float64
	switch product.FreeQuotaInterval {
	case FreeQuotaMonthly:
		unitPrice = product.Price * float64(product.UnitSize) / numDaysPerMonth
	case FreeQuotaDaily:
		unitPrice = product.Price * float64(product.UnitSize)
	}
	return math.Floor(unitPrice*unitPricePrecision) / unitPricePrecision
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

func (s *Service) createPrice(client *stripec.API, productName string, unitPrice float64) (string, error) {
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
			Interval:       stripe.String(string(stripe.PriceRecurringIntervalMonth)),
			IntervalCount:  stripe.Int64(1),
			UsageType:      stripe.String(string(stripe.PriceRecurringUsageTypeMetered)),
		},
		Tiers: []*stripe.PriceTierParams{
			{
				UpToInf:           stripe.Bool(true),
				UnitAmountDecimal: stripe.Float64(unitPrice),
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

func (s *Service) Stop() error {
	rctx := s.reporter.Stop()
	<-rctx.Done()
	log.Info("reporter was shutdown")

	s.semaphores.Stop()
	log.Info("locking customers")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := s.gateway.Stop(); err != nil {
		return err
	}

	stopped := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(stopped)
	}()
	timer := time.NewTimer(10 * time.Second)
	select {
	case <-timer.C:
		s.server.Stop()
	case <-stopped:
		timer.Stop()
	}
	log.Info("gRPC was shutdown")

	return s.cdb.Database().Client().Disconnect(ctx)
}

func (s *Service) CheckHealth(_ context.Context, _ *pb.CheckHealthRequest) (*pb.CheckHealthResponse, error) {
	log.Debugf("health check okay")
	return &pb.CheckHealthResponse{}, nil
}

func (s *Service) CreateCustomer(ctx context.Context, req *pb.CreateCustomerRequest) (
	*pb.CreateCustomerResponse, error) {
	var parentKey string
	if req.Parent != nil {
		if _, err := s.createCustomer(ctx, req.Parent, ""); err != nil &&
			!errors.Is(err, ErrCustomerExists) {
			return nil, err
		}
		parentKey = req.Parent.Key
	}
	cus, err := s.createCustomer(ctx, req.Customer, parentKey)
	if err != nil {
		return nil, err
	}
	return &pb.CreateCustomerResponse{
		CustomerId: cus.CustomerID,
	}, nil
}

func (s *Service) createCustomer(
	ctx context.Context,
	params *pb.CreateCustomerRequest_Params,
	parentKey string,
) (*Customer, error) {
	lck := s.semaphores.Get(customerLock(params.Key))
	lck.Acquire()
	defer lck.Release()

	doc, err := s.getCustomer(ctx, "_id", params.Key)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	} else if err == nil {
		return nil, ErrCustomerExists
	}

	customer, err := s.stripe.Customers.New(&stripe.CustomerParams{
		Email: stripe.String(params.Email),
	})
	if err != nil {
		return nil, err
	}

	doc = &Customer{
		Key:         params.Key,
		CustomerID:  customer.ID,
		ParentKey:   parentKey,
		Email:       params.Email,
		AccountType: mdb.AccountType(params.AccountType),
		CreatedAt:   time.Now().Unix(),
	}
	if err := s.createSubscription(doc); err != nil {
		return nil, err
	}
	if _, err := s.cdb.InsertOne(ctx, doc); err != nil {
		return nil, err
	}
	log.Debugf("created customer %s with id %s", doc.Key, doc.CustomerID)

	s.analytics.Identify(doc.Key, doc.AccountType, false, doc.Email, map[string]interface{}{
		"parent_key":  doc.ParentKey,
		"customer_id": doc.CustomerID,
		"username":    params.Username,
	})

	return doc, nil
}

func (s *Service) getCustomer(ctx context.Context, key, val string) (*Customer, error) {
	r := s.cdb.FindOne(ctx, bson.M{key: val})
	if r.Err() != nil {
		return nil, r.Err()
	}
	var doc Customer
	return &doc, r.Decode(&doc)
}

func (s *Service) createSubscription(cus *Customer) error {
	var prices []*stripe.SubscriptionItemsParams
	for _, p := range s.products {
		prices = append(prices,
			&stripe.SubscriptionItemsParams{
				Price: stripe.String(p.FreePriceID),
			},
			&stripe.SubscriptionItemsParams{
				Price: stripe.String(p.PaidPriceID),
			},
		)
	}
	sub, err := s.stripe.Subscriptions.New(&stripe.SubscriptionParams{
		Customer: stripe.String(cus.CustomerID),
		Items:    prices,
	})
	if err != nil {
		return err
	}
	cus.SubscriptionStatus = string(sub.Status)
	cus.InvoicePeriod = Period{
		UnixStart: sub.CurrentPeriodStart,
		UnixEnd:   sub.CurrentPeriodEnd,
	}
	if cus.DailyUsage == nil {
		cus.DailyUsage = make(map[string]Usage)
	}
	for _, item := range sub.Items.Data {
		for k, p := range s.products {
			var u Usage
			if _, ok := cus.DailyUsage[k]; ok {
				u = cus.DailyUsage[k]
			}
			switch item.Price.ID {
			case p.FreePriceID:
				u.FreeItemID = item.ID
			case p.PaidPriceID:
				u.PaidItemID = item.ID
			default:
				continue
			}
			cus.DailyUsage[k] = u
		}
	}
	return nil
}

func (s *Service) GetCustomer(ctx context.Context, req *pb.GetCustomerRequest) (
	*pb.GetCustomerResponse, error) {
	doc, err := s.getCustomer(ctx, "_id", req.Key)
	if err != nil {
		return nil, err
	}
	log.Debugf("got customer %s", doc.Key)
	return s.customerToPb(ctx, doc)
}

func periodToPb(period Period) *pb.Period {
	return &pb.Period{
		UnixStart: period.UnixStart,
		UnixEnd:   period.UnixEnd,
	}
}

func (s *Service) dailyUsageToPb(usage map[string]Usage) map[string]*pb.Usage {
	start, end := getCurrentDayBounds()
	res := make(map[string]*pb.Usage)
	for k, u := range usage {
		if product, ok := s.products[k]; ok {
			res[k] = getUsage(product, u.Total, Period{UnixStart: start, UnixEnd: end})
		}
	}
	return res
}

func getCurrentDayBounds() (int64, int64) {
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	end := time.Date(now.Year(), now.Month(), now.Day(), 24, 0, 0, 0, time.Local)
	return start.Unix(), end.Unix()
}

func getCost(product Product, paidUnits int64) float64 {
	if paidUnits > 0 {
		return float64(paidUnits) * getUnitPrice(product)
	}
	return 0
}

func getUsage(product Product, total int64, period Period) *pb.Usage {
	freeUnits, paidUnits := getUnits(product, total)
	free := product.FreeQuotaSize - total
	if free < 0 {
		free = 0
	}
	grace := product.FreeQuotaGracePeriodSize - total
	if free < 0 {
		free = 0
	}
	return &pb.Usage{
		Description: product.Name,
		Units:       freeUnits + paidUnits,
		Total:       total,
		Free:        free,
		Grace:       grace,
		Cost:        getCost(product, paidUnits),
		Period:      periodToPb(period),
	}
}

func getUnits(product Product, total int64) (freeUnits, paidUnits int64) {
	var freeSize, paidSize int64
	if total > product.FreeQuotaSize {
		freeSize = product.FreeQuotaSize
		paidSize = total - product.FreeQuotaSize
	} else {
		freeSize = total
	}
	freeUnits = int64(math.Round(float64(freeSize) / float64(product.UnitSize)))
	paidUnits = int64(math.Round(float64(paidSize) / float64(product.UnitSize)))
	return
}

func addProductToSummary(summary map[string]interface{}, product Product, total int64) {
	_, paidUnits := getUnits(product, total)
	cost := getCost(product, paidUnits)
	summary[product.Key+"_name"] = product.Name
	if product.Units != "" {
		summary[product.Key+"_units"] = product.Units
	}
	summary[product.Key+"_free_quota_size"] = product.FreeQuotaSize
	summary[product.Key+"_total"] = total
	summary[product.Key+"_cost"] = cost
}

func (s *Service) getSummary(cus *Customer, deps int64) map[string]interface{} {
	summary := map[string]interface{}{
		"account_status":       cus.AccountStatus(),
		"billable":             cus.Billable,
		"delinquent":           cus.Delinquent,
		"grace_period_start":   s.analytics.FormatUnix(cus.GracePeriodStart),
		"invoice_period_end":   s.analytics.FormatUnix(cus.InvoicePeriod.UnixEnd),
		"invoice_period_start": s.analytics.FormatUnix(cus.InvoicePeriod.UnixStart),
		"subscription_status":  cus.SubscriptionStatus,
	}
	if cus.GracePeriodStart > 0 {
		summary["grace_period_end"] = s.analytics.FormatUnix(cus.GracePeriodStart + int64(s.config.FreeQuotaGracePeriod.Seconds()))
	}
	if deps > 0 {
		summary["dependents"] = deps
	}
	return summary
}

func (s *Service) customerToPb(ctx context.Context, doc *Customer) (*pb.GetCustomerResponse, error) {
	deps, err := s.cdb.CountDocuments(ctx, bson.M{"parent_key": doc.Key})
	if err != nil {
		return nil, err
	}
	var gracePeriodEnd int64
	if doc.GracePeriodStart > 0 {
		gracePeriodEnd = doc.GracePeriodStart + int64(s.config.FreeQuotaGracePeriod.Seconds())
	}
	return &pb.GetCustomerResponse{
		Key:                doc.Key,
		CustomerId:         doc.CustomerID,
		ParentKey:          doc.ParentKey,
		Email:              doc.Email,
		AccountType:        int32(doc.AccountType),
		AccountStatus:      doc.AccountStatus(),
		SubscriptionStatus: doc.SubscriptionStatus,
		Balance:            doc.Balance,
		Billable:           doc.Billable,
		Delinquent:         doc.Delinquent,
		CreatedAt:          doc.CreatedAt,
		GracePeriodEnd:     gracePeriodEnd,
		InvoicePeriod:      periodToPb(doc.InvoicePeriod),
		DailyUsage:         s.dailyUsageToPb(doc.DailyUsage),
		Dependents:         deps,
	}, nil
}

func (s *Service) GetCustomerSession(ctx context.Context, req *pb.GetCustomerSessionRequest) (
	*pb.GetCustomerSessionResponse, error) {
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
	cursor, err := s.cdb.Find(ctx, filter, opts)
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

	if _, err := s.cdb.UpdateOne(ctx, bson.M{"_id": doc.Key}, bson.M{
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

	update := bson.M{
		"subscription_status":       req.Status,
		"invoice_period.unix_start": req.InvoicePeriod.UnixStart,
		"invoice_period.unix_end":   req.InvoicePeriod.UnixEnd,
	}
	if doc.InvoicePeriod.UnixEnd < req.InvoicePeriod.UnixEnd {
		for k := range doc.DailyUsage {
			if product, ok := s.products[k]; ok {
				if product.FreeQuotaInterval == FreeQuotaMonthly &&
					product.PriceType == PriceTypeIncremental {
					update["daily_usage."+k+".total"] = 0
				}
			}
		}
	}
	if _, err := s.cdb.UpdateOne(ctx, bson.M{"_id": doc.Key}, bson.M{"$set": update}); err != nil {
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
	if _, err := s.cdb.UpdateOne(ctx, bson.M{"_id": req.Key}, bson.M{
		"$set": bson.M{
			"subscription_status": doc.SubscriptionStatus,
			"invoice_period":      doc.InvoicePeriod,
			"daily_usage":         doc.DailyUsage,
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
	if _, err := s.cdb.DeleteOne(ctx, bson.M{"_id": req.Key}); err != nil {
		return nil, err
	}
	log.Debugf("deleted customer %s", req.Key)
	return &pb.DeleteCustomerResponse{}, nil
}

func (s *Service) GetCustomerUsage(
	ctx context.Context,
	req *pb.GetCustomerUsageRequest,
) (*pb.GetCustomerUsageResponse, error) {
	doc, err := s.getCustomer(ctx, "_id", req.Key)
	if err != nil {
		return nil, err
	}
	usage := make(map[string]*pb.Usage)
	for k, u := range doc.DailyUsage {
		if product, ok := s.products[k]; ok {
			// Add current day unreported usage
			total := u.Total
			// Get reported usage over the current invoice period
			if product.FreeQuotaInterval == FreeQuotaDaily &&
				product.PriceType == PriceTypeIncremental {
				free, err := s.getPeriodUsageItem(u.FreeItemID)
				if err != nil {
					return nil, err
				}
				paid, err := s.getPeriodUsageItem(u.PaidItemID)
				if err != nil {
					return nil, err
				}
				total += (free.TotalUsage + paid.TotalUsage) * product.UnitSize
			}
			usage[k] = getUsage(product, total, doc.InvoicePeriod)
		}
	}
	return &pb.GetCustomerUsageResponse{
		Usage: usage,
	}, nil
}

func (s *Service) getPeriodUsageItem(id string) (sum *stripe.UsageRecordSummary, err error) {
	params := &stripe.UsageRecordSummaryListParams{
		SubscriptionItem: stripe.String(id),
	}
	params.Filters.AddFilter("limit", "", "1")
	params.Single = true
	i := s.stripe.UsageRecordSummaries.List(params)
	for i.Next() {
		sum = i.UsageRecordSummary()
	}
	if i.Err() != nil {
		return nil, i.Err()
	}
	if sum != nil && sum.Period != nil {
		return sum, nil
	}
	return nil, fmt.Errorf("subscription item %s not found", id)
}

func (s *Service) IncCustomerUsage(
	ctx context.Context,
	req *pb.IncCustomerUsageRequest,
) (*pb.IncCustomerUsageResponse, error) {
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

	res := &pb.IncCustomerUsageResponse{
		DailyUsage: make(map[string]*pb.Usage),
	}
	for k, inc := range req.ProductUsage {
		if product, ok := s.products[k]; ok && inc != 0 {
			usage, err := s.handleUsage(ctx, cus, product, inc)
			if err != nil {
				return nil, err
			}
			if usage != nil {
				log.Debugf("%s %s: total=%d free=%d", cus.Key, k, usage.Total, usage.Free)
				res.DailyUsage[k] = usage
			}
		}
	}
	return res, nil
}

func (s *Service) handleUsage(ctx context.Context, cus *Customer, product Product, incSize int64) (*pb.Usage, error) {
	usage, ok := cus.DailyUsage[product.Key]
	if !ok {
		return nil, nil
	}
	total := usage.Total + incSize
	if total < 0 {
		log.Warnf("negative %s detected: total=%d inc=%d)", product.Key, total, incSize)
		total = 0
	}
	update := bson.M{"daily_usage." + product.Key + ".total": total}

	if total > product.FreeQuotaSize && !cus.Billable {
		now := time.Now().Unix()
		if cus.GracePeriodStart == 0 {
			cus.GracePeriodStart = now
			update["grace_period_start"] = cus.GracePeriodStart
			summary := s.getSummary(cus, 0)
			addProductToSummary(summary, product, total)
			s.analytics.TrackEvent(cus.Key, cus.AccountType, false, analytics.GracePeriodStart, nil)
		}
		deadline := cus.GracePeriodStart + int64(s.config.FreeQuotaGracePeriod.Seconds())
		if now >= deadline {
			summary := s.getSummary(cus, 0)
			addProductToSummary(summary, product, total)
			s.analytics.TrackEvent(cus.Key, cus.AccountType, false, analytics.GracePeriodEnd, nil)
			return nil, common.ErrExceedsFreeQuota
		}
	}
	if _, err := s.cdb.UpdateOne(ctx, bson.M{"_id": cus.Key}, bson.M{"$set": update}); err != nil {
		return nil, err
	}
	start, end := getCurrentDayBounds()
	return getUsage(product, total, Period{UnixStart: start, UnixEnd: end}), nil
}

func (s *Service) ReportCustomerUsage(
	ctx context.Context,
	req *pb.ReportCustomerUsageRequest,
) (*pb.ReportCustomerUsageResponse, error) {
	doc, err := s.getCustomer(ctx, "_id", req.Key)
	if err != nil {
		return nil, err
	}
	if err := s.reportCustomerUsage(ctx, doc); err != nil {
		return nil, err
	}
	return &pb.ReportCustomerUsageResponse{}, nil
}

func (s *Service) reportUsage() error {
	ctx, cancel := context.WithTimeout(context.Background(), reporterTimeout)
	defer cancel()
	cursor, err := s.cdb.Find(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("finding customers: %v", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var doc Customer
		if err := cursor.Decode(&doc); err != nil {
			return fmt.Errorf("decoding customer: %v", err)
		}
		if err := s.reportCustomerUsage(ctx, &doc); err != nil {
			return fmt.Errorf("reporting customer usage: %v", err)
		}
	}
	if err := cursor.Err(); err != nil {
		return fmt.Errorf("cursor error: %v", err)
	}
	return nil
}

func (s *Service) reportCustomerUsage(ctx context.Context, cus *Customer) error {
	deps, err := s.cdb.CountDocuments(ctx, bson.M{"parent_key": cus.Key})
	if err != nil {
		return err
	}
	summary := s.getSummary(cus, deps)

	for k, usage := range cus.DailyUsage {
		if product, ok := s.products[k]; ok {
			if err := s.reportUnits(product, usage, cus.ParentKey); err != nil {
				return err
			}

			addProductToSummary(summary, product, usage.Total)
			log.Debugf("reported usage for %s: %s=%d", cus.Key, k, usage.Total)
			if product.FreeQuotaInterval == FreeQuotaDaily &&
				product.PriceType == PriceTypeIncremental {
				if _, err := s.cdb.UpdateOne(ctx, bson.M{"_id": cus.Key}, bson.M{
					"$set": bson.M{"daily_usage." + k + ".total": 0},
				}); err != nil {
					return err
				}
			}
		} else {
			log.Warn("%s has invalid product key: %s", cus.Key, k)
		}
	}
	s.analytics.Identify(cus.Key, cus.AccountType, false, "", summary)
	return nil
}

func (s *Service) reportUnits(product Product, usage Usage, parentKey string) error {
	freeUnits, paidUnits := getUnits(product, usage.Total)
	if parentKey != "" {
		freeUnits += paidUnits
		paidUnits = 0
	}
	if freeUnits > 0 {
		if _, err := s.stripe.UsageRecords.New(&stripe.UsageRecordParams{
			SubscriptionItem: stripe.String(usage.FreeItemID),
			Quantity:         stripe.Int64(freeUnits),
			Timestamp:        stripe.Int64(time.Now().Unix()),
			Action:           stripe.String(stripe.UsageRecordActionIncrement),
		}); err != nil {
			return err
		}
	}
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

// Identify creates or updates the user traits
func (s *Service) Identify(
	ctx context.Context,
	req *pb.IdentifyRequest,
) (*pb.IdentifyResponse, error) {
	props := map[string]interface{}{}
	for k, v := range req.Properties {
		props[k] = v
	}
	err := s.analytics.Identify(
		req.Key,
		mdb.AccountType(req.AccountType),
		req.Active,
		req.Email,
		props,
	)
	if err != nil {
		return nil, err
	}
	return &pb.IdentifyResponse{}, nil
}

// TrackEvent records a new event
func (s *Service) TrackEvent(
	ctx context.Context,
	req *pb.TrackEventRequest,
) (*pb.TrackEventResponse, error) {
	err := s.analytics.TrackEvent(
		req.Key,
		mdb.AccountType(req.AccountType),
		req.Active,
		analytics.Event(req.Event),
		req.Properties,
	)
	if err != nil {
		return nil, err
	}
	return &pb.TrackEventResponse{}, nil
}
