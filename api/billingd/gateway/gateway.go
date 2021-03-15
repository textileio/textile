package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	logging "github.com/ipfs/go-log/v2"
	ma "github.com/multiformats/go-multiaddr"
	stripe "github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/webhook"
	tutil "github.com/textileio/go-threads/util"
	billing "github.com/textileio/textile/v2/api/billingd/client"
	"github.com/textileio/textile/v2/api/common"
	"go.mongodb.org/mongo-driver/mongo"
	"google.golang.org/grpc"
)

var log = logging.Logger("billing.gateway")

const handlerTimeout = time.Minute

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// Gateway provides an endpoint for Stripe webhooks.
type Gateway struct {
	addr                ma.Multiaddr
	server              *http.Server
	client              *billing.Client
	stripeWebhookSecret string
}

// Config defines the gateway configuration.
type Config struct {
	Addr                ma.Multiaddr
	APIAddr             ma.Multiaddr
	StripeWebhookSecret string
	SegmentAPIKey       string
	Debug               bool
}

// NewGateway returns a new gateway.
func NewGateway(conf Config) (*Gateway, error) {
	if conf.Debug {
		if err := tutil.SetLogLevels(map[string]logging.LogLevel{
			"billing.gateway": logging.LevelDebug,
		}); err != nil {
			return nil, err
		}
	}
	apiTarget, err := tutil.TCPAddrFromMultiAddr(conf.APIAddr)
	if err != nil {
		return nil, err
	}
	client, err := billing.NewClient(
		apiTarget,
		grpc.WithInsecure(),
		grpc.WithPerRPCCredentials(common.Credentials{}),
	)
	if err != nil {
		return nil, err
	}
	return &Gateway{
		addr:                conf.Addr,
		client:              client,
		stripeWebhookSecret: conf.StripeWebhookSecret,
	}, nil
}

// Start the gateway.
func (g *Gateway) Start() {
	addr, err := tutil.TCPAddrFromMultiAddr(g.addr)
	if err != nil {
		log.Fatal(err)
	}

	router := gin.Default()
	router.GET("/health", func(c *gin.Context) {
		c.Writer.WriteHeader(http.StatusNoContent)
	})
	router.POST("/webhooks", g.webhookHandler)

	g.server = &http.Server{
		Addr:    addr,
		Handler: router,
	}
	go func() {
		if err := g.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("gateway error: %s", err)
		}
		log.Info("gateway was shutdown")
	}()
	log.Infof("gateway listening at %s", g.server.Addr)
}

// Addr returns the gateway's address.
func (g *Gateway) Addr() string {
	return g.server.Addr
}

// Stop the gateway.
func (g *Gateway) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := g.server.Shutdown(ctx); err != nil {
		return err
	}
	if err := g.client.Close(); err != nil {
		return err
	}
	return nil
}

const webhookMaxBodyBytes = int64(65536) // 64 KiB

// webhookHandler handles stripe webhook events
func (g *Gateway) webhookHandler(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, webhookMaxBodyBytes)
	payload, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		log.Errorf("reading request body: %v", err)
		c.Status(http.StatusBadRequest)
		return
	}

	event := stripe.Event{}
	if g.stripeWebhookSecret != "" {
		event, err = webhook.ConstructEvent(payload, c.Request.Header.Get("Stripe-Signature"), g.stripeWebhookSecret)
		if err != nil {
			log.Errorf("verifying webhook signature: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}
	}

	log.Debugf("received event type: %s", event.Type)
	switch event.Type {
	case "customer.updated":
		var cus stripe.Customer
		if err := json.Unmarshal(event.Data.Raw, &cus); err != nil {
			log.Errorf("parsing webhook JSON: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}
		var billable bool
		if !cus.Deleted {
			if cus.InvoiceSettings.DefaultPaymentMethod != nil {
				billable = true
			} else if cus.DefaultSource != nil && !cus.DefaultSource.Deleted {
				billable = true
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
		defer cancel()
		if err := g.client.UpdateCustomer(ctx, cus.ID, cus.Balance, billable, cus.Delinquent); err != nil {
			// This webhook receives events from all deployments (production and staging),
			// which leads to a lot of "customer not found" errors.
			// To avoid this, we'll need a Stripe account for each deployment.
			// See https://github.com/textileio/textile/issues/523.
			if !errors.Is(err, mongo.ErrNoDocuments) {
				log.Errorf("updating customer: %v", err)
			}
			c.Status(http.StatusOK)
			return
		}
		log.Debugf("%s was updated", cus.ID)
	case "customer.deleted":
		// @todo: Does this need to be handled?
	case "customer.subscription.updated",
		"customer.subscription.deleted":
		var sub stripe.Subscription
		err := json.Unmarshal(event.Data.Raw, &sub)
		if err != nil {
			log.Errorf("parsing webhook JSON: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), handlerTimeout)
		defer cancel()
		if err := g.client.UpdateCustomerSubscription(
			ctx,
			sub.Customer.ID,
			sub.Status,
			sub.CurrentPeriodStart,
			sub.CurrentPeriodEnd,
		); err != nil {
			// This webhook receives events from all deployments (production and staging),
			// which leads to a lot of "customer not found" errors.
			// To avoid this, we'll need a Stripe account for each deployment.
			// See https://github.com/textileio/textile/issues/523.
			if !errors.Is(err, mongo.ErrNoDocuments) {
				log.Errorf("updating customer subscription: %v", err)
			}
			c.Status(http.StatusOK)
			return
		}
		log.Debugf("%s subscription was updated", sub.ID)
	}

	c.Status(http.StatusOK)
}
