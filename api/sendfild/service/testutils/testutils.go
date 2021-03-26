package testutils

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/lotus/api/apistruct"
	"github.com/stretchr/testify/require"
	"github.com/textileio/powergate/v2/tests"
	pb "github.com/textileio/textile/v2/api/sendfild/pb"
	service "github.com/textileio/textile/v2/api/sendfild/service"
	"github.com/textileio/textile/v2/api/sendfild/service/interfaces"
	"github.com/textileio/textile/v2/util"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

const (
	bufSize = 1024 * 1024
)

type setupConfig struct {
	dbName             string
	messageWaitTimeout time.Duration
	messageConfidence  uint64
	retryWaitFrequency time.Duration
	speed              int
}

type SetupOption = func(*setupConfig)

func SetupWithSpeed(speed int) SetupOption {
	return func(config *setupConfig) {
		config.speed = speed
	}
}

func SetupWithDbName(dbName string) SetupOption {
	return func(config *setupConfig) {
		config.dbName = dbName
	}
}

func SetupWithMessageWaitTimeout(messageWaitTimeout time.Duration) SetupOption {
	return func(config *setupConfig) {
		config.messageWaitTimeout = messageWaitTimeout
	}
}

func SetupWithMessageConfidence(messageConfidence uint64) SetupOption {
	return func(config *setupConfig) {
		config.messageConfidence = messageConfidence
	}
}

func SetupWithRetryWaitFrequency(retryWaitFrequency time.Duration) SetupOption {
	return func(config *setupConfig) {
		config.retryWaitFrequency = retryWaitFrequency
	}
}

func RequireSetupLotus(t *testing.T, ctx context.Context, opts ...SetupOption) (interfaces.FilecoinClientBuilder, *apistruct.FullNodeStruct, address.Address, func()) {
	config := &setupConfig{
		speed: 300,
	}
	for _, opt := range opts {
		opt(config)
	}
	cb, addr, _ := tests.CreateLocalDevnet(t, 1, config.speed)
	time.Sleep(time.Millisecond * 500) // Allow the network to some tipsets

	lotusClient, closeLotusClient, err := cb(ctx)
	require.NoError(t, err)

	cleanup := func() {
		closeLotusClient()
	}

	clientBuilder := func(ctx context.Context) (interfaces.FilecoinClient, func(), error) {
		return cb(ctx)
	}
	return clientBuilder, lotusClient, addr, cleanup
}

func RequireSetupService(t *testing.T, ctx context.Context, cb interfaces.FilecoinClientBuilder, opts ...SetupOption) (pb.SendFilServiceClient, *grpc.ClientConn, func()) {
	config := &setupConfig{
		dbName:             util.MakeToken(12),
		messageWaitTimeout: time.Minute,
		messageConfidence:  2,
		retryWaitFrequency: time.Minute,
	}
	for _, opt := range opts {
		opt(config)
	}
	listener := bufconn.Listen(bufSize)

	conf := service.Config{
		Listener:            listener,
		ClientBuilder:       cb,
		MessageWaitTimeout:  config.messageWaitTimeout,
		MessageConfidence:   config.messageConfidence,
		RetryWaitFrequency:  config.retryWaitFrequency,
		Debug:               true,
		AllowEmptyFromAddrs: true,
	}
	s, err := service.New(conf)
	require.NoError(t, err)

	bufDialer := func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}

	clientConn, err := grpc.Dial("bufnet", grpc.WithContextDialer(bufDialer), grpc.WithInsecure())
	require.NoError(t, err)
	client := pb.NewSendFilServiceClient(clientConn)

	cleanup := func() {
		clientConn.Close()
		s.Close()
	}

	return client, clientConn, cleanup
}

func RequireSetup(t *testing.T, ctx context.Context, opts ...SetupOption) (pb.SendFilServiceClient, *grpc.ClientConn, *apistruct.FullNodeStruct, address.Address, func()) {
	cb, lotusClient, addr, cleanupLouts := RequireSetupLotus(t, ctx, opts...)
	serviceClient, clientConn, cleanupService := RequireSetupService(t, ctx, cb, opts...)

	cleanup := func() {
		cleanupLouts()
		cleanupService()
	}

	return serviceClient, clientConn, lotusClient, addr, cleanup
}
