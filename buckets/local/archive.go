package local

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/textileio/textile/v2/api/buckets/client"
	pb "github.com/textileio/textile/v2/api/buckets/pb"
)

// ArchiveStatusTimeout is the timeout used when requesting a single status message.
var ArchiveStatusTimeout = time.Second * 5

// ArchiveConfig is the desired state of a Cid in the Filecoin network.
type ArchiveConfig struct {
	// RepFactor (ignored in Filecoin mainnet) indicates the desired amount of active deals
	// with different miners to store the data. While making deals
	// the other attributes of FilConfig are considered for miner selection.
	RepFactor int
	// DealMinDuration indicates the duration to be used when making new deals.
	DealMinDuration int64
	// ExcludedMiners (ignored in Filecoin mainnet) is a set of miner addresses won't be ever be selected
	// when making new deals, even if they comply to other filters.
	ExcludedMiners []string
	// TrustedMiners (ignored in Filecoin mainnet) is a set of miner addresses which will be forcibly used
	// when making new deals. An empty/nil list disables this feature.
	TrustedMiners []string
	// CountryCodes (ignored in Filecoin mainnet) indicates that new deals should select miners on specific
	// countries.
	CountryCodes []string
	// Renew indicates deal-renewal configuration.
	Renew ArchiveRenew
	// Addr is the wallet address used to store the data in filecoin
	Addr string
	// MaxPrice is the maximum price that will be spent to store the data
	MaxPrice uint64
	// FastRetrieval indicates that created deals should enable the
	// fast retrieval feature.
	FastRetrieval bool
	// DealStartOffset indicates how many epochs in the future impose a
	// deadline to new deals being active on-chain. This value might influence
	// if miners accept deals, since they should seal fast enough to satisfy
	// this constraint.
	DealStartOffset int64
}

// ArchiveRenew contains renew configuration for a ArchiveConfig.
type ArchiveRenew struct {
	// Enabled indicates that deal-renewal is enabled for this Cid.
	Enabled bool
	// Threshold indicates how many epochs before expiring should trigger
	// deal renewal. e.g: 100 epoch before expiring.
	Threshold int
}

// DefaultArchiveConfig gets the default archive config for the specified Bucket.
func (b *Bucket) DefaultArchiveConfig(ctx context.Context) (config ArchiveConfig, err error) {
	b.Lock()
	defer b.Unlock()
	ctx, err = b.context(ctx)
	if err != nil {
		return
	}
	pbConfig, err := b.clients.Buckets.DefaultArchiveConfig(ctx, b.Key())
	if err != nil {
		return
	}
	if pbConfig == nil {
		return config, fmt.Errorf("no archive config in response")
	}
	config = fromPbArchiveConfig(*pbConfig)
	return
}

func fromPbArchiveConfig(pbConfig pb.ArchiveConfig) ArchiveConfig {
	config := ArchiveConfig{
		RepFactor:       int(pbConfig.RepFactor),
		DealMinDuration: pbConfig.DealMinDuration,
		ExcludedMiners:  pbConfig.ExcludedMiners,
		TrustedMiners:   pbConfig.TrustedMiners,
		CountryCodes:    pbConfig.CountryCodes,
		Addr:            pbConfig.Addr,
		MaxPrice:        pbConfig.MaxPrice,
		FastRetrieval:   pbConfig.FastRetrieval,
		DealStartOffset: pbConfig.DealStartOffset,
	}
	if pbConfig.Renew != nil {
		config.Renew = ArchiveRenew{
			Enabled:   pbConfig.Renew.Enabled,
			Threshold: int(pbConfig.Renew.Threshold),
		}
	}
	return config
}

// SetDefaultArchiveConfig sets the default archive config for the specified Bucket.
func (b *Bucket) SetDefaultArchiveConfig(ctx context.Context, config ArchiveConfig) (err error) {
	b.Lock()
	defer b.Unlock()
	ctx, err = b.context(ctx)
	if err != nil {
		return
	}
	err = b.clients.Buckets.SetDefaultArchiveConfig(ctx, b.Key(), toPbArchiveConfig(config))
	return
}

func toPbArchiveConfig(config ArchiveConfig) *pb.ArchiveConfig {
	return &pb.ArchiveConfig{
		RepFactor:       int32(config.RepFactor),
		DealMinDuration: config.DealMinDuration,
		ExcludedMiners:  config.ExcludedMiners,
		TrustedMiners:   config.TrustedMiners,
		CountryCodes:    config.CountryCodes,
		Renew: &pb.ArchiveRenew{
			Enabled:   config.Renew.Enabled,
			Threshold: int32(config.Renew.Threshold),
		},
		Addr:            config.Addr,
		MaxPrice:        config.MaxPrice,
		FastRetrieval:   config.FastRetrieval,
		DealStartOffset: config.DealStartOffset,
	}
}

type archiveRemoteOptions struct {
	archiveConfig *ArchiveConfig
}

type ArchiveRemoteOption func(*archiveRemoteOptions)

// WithArchiveConfig allows you to provide a custom ArchiveConfig for a single call to ArchiveRemote.
func WithArchiveConfig(config ArchiveConfig) ArchiveRemoteOption {
	return func(opts *archiveRemoteOptions) {
		opts.archiveConfig = &config
	}
}

// ArchiveRemote requests an archive of the current remote bucket.
func (b *Bucket) ArchiveRemote(ctx context.Context, opts ...ArchiveRemoteOption) error {
	b.Lock()
	defer b.Unlock()

	options := &archiveRemoteOptions{}
	for _, opt := range opts {
		opt(options)
	}

	var clientOpts []client.ArchiveOption
	if options.archiveConfig != nil {
		clientOpts = append(clientOpts, client.WithArchiveConfig(toPbArchiveConfig(*options.archiveConfig)))
	}

	ctx, err := b.context(ctx)
	if err != nil {
		return err
	}

	return b.clients.Buckets.Archive(ctx, b.Key(), clientOpts...)
}

// ArchiveStatusMessage is used to wrap an archive status message.
type ArchiveStatusMessage struct {
	Type            ArchiveMessageType
	Message         string
	Error           error
	InactivityClose bool
}

// ArchiveMessageType is the type of status message.
type ArchiveMessageType int

const (
	// ArchiveMessage accompanies an informational message.
	ArchiveMessage ArchiveMessageType = iota
	// ArchiveWarning accompanies a warning state.
	ArchiveWarning
	// ArchiveError accompanies an error state.
	ArchiveError
	// ArchiveSuccess accompanies a successful state.
	ArchiveSuccess
)

// ArchiveStatus returns the current archive status.
// When watch is true, the channel remains open, delivering all messages.
func (b *Bucket) ArchiveStatus(ctx context.Context, watch bool) (<-chan ArchiveStatusMessage, error) {
	b.Lock()
	defer b.Unlock()
	ctx, err := b.context(ctx)
	if err != nil {
		return nil, err
	}
	key := b.Key()
	rep, err := b.clients.Buckets.ArchiveStatus(ctx, key)
	if err != nil {
		return nil, err
	}
	msgs := make(chan ArchiveStatusMessage)
	go func() {
		defer close(msgs)
		switch rep.GetStatus() {
		case pb.ArchiveStatusResponse_STATUS_FAILED:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveWarning,
				Message: "Archive failed with message: " + rep.GetFailedMsg(),
			}
		case pb.ArchiveStatusResponse_STATUS_CANCELED:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveWarning,
				Message: "Archive was superseded by a new executing archive",
			}
		case pb.ArchiveStatusResponse_STATUS_EXECUTING:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveMessage,
				Message: "Archive is currently executing, grab a coffee and be patient...",
			}
		case pb.ArchiveStatusResponse_STATUS_DONE:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveSuccess,
				Message: "Archive executed successfully!",
			}
		default:
			msgs <- ArchiveStatusMessage{
				Type:    ArchiveWarning,
				Message: "Archive status unknown",
			}
		}
		if watch {
			ch := make(chan string)
			wCtx, cancel := context.WithCancel(ctx)
			defer cancel()
			var err error
			go func() {
				err = b.clients.Buckets.ArchiveWatch(wCtx, key, ch)
				close(ch)
			}()
			for msg := range ch {
				msgs <- ArchiveStatusMessage{Type: ArchiveMessage, Message: "\t " + msg}
			}
			if err != nil {
				if strings.Contains(err.Error(), "RST_STREAM") {
					msgs <- ArchiveStatusMessage{Type: ArchiveError, InactivityClose: true}
					return
				}
				msgs <- ArchiveStatusMessage{Type: ArchiveError, Error: err}
			}
		}
	}()
	return msgs, nil
}

// ArchiveInfo wraps info about an archive.
type ArchiveInfo struct {
	Key     string  `json:"key"`
	Archive Archive `json:"archive"`
}

// Archive describes the state of an archive.
type Archive struct {
	Cid   cid.Cid       `json:"cid"`
	Deals []ArchiveDeal `json:"deals"`
}

// ArchiveDeal describes an archive deal.
type ArchiveDeal struct {
	ProposalCid cid.Cid `json:"proposal_cid"`
	Miner       string  `json:"miner"`
}

// ArchiveInfo returns information about the current archvie.
func (b *Bucket) ArchiveInfo(ctx context.Context) (info ArchiveInfo, err error) {
	b.Lock()
	defer b.Unlock()
	ctx, err = b.context(ctx)
	if err != nil {
		return
	}
	rep, err := b.clients.Buckets.ArchiveInfo(ctx, b.Key())
	if err != nil {
		return
	}
	return pbArchiveInfoToArchiveInfo(rep)
}

func pbArchiveInfoToArchiveInfo(pi *pb.ArchiveInfoResponse) (info ArchiveInfo, err error) {
	info.Key = pi.Key
	if pi.Archive != nil {
		info.Archive.Cid, err = cid.Decode(pi.Archive.Cid)
		if err != nil {
			return
		}
		deals := make([]ArchiveDeal, len(pi.Archive.Deals))
		for i, d := range pi.Archive.Deals {
			deals[i].Miner = d.Miner
			deals[i].ProposalCid, err = cid.Decode(d.ProposalCid)
			if err != nil {
				return
			}
		}
		info.Archive.Deals = deals
	}
	return info, err
}
