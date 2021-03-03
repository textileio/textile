package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type BucketArchive struct {
	BucketKey            string         `bson:"_id"`
	Archives             Archives       `bson:"archives"`
	DefaultArchiveConfig *ArchiveConfig `bson:"default_archive_config"`
}

type Archives struct {
	Current Archive   `bson:"current"`
	History []Archive `bson:"history"`
}

type Archive struct {
	Cid        []byte     `bson:"cid"`
	JobID      string     `bson:"job_id"`
	Status     int        `bson:"status"`
	Aborted    bool       `bson:"aborted"`
	AbortedMsg string     `bson:"aborted_msg"`
	FailureMsg string     `bson:"failure_msg"`
	CreatedAt  int64      `bson:"created_at"`
	DealInfo   []DealInfo `bson:"deal_info"`
}

type DealInfo struct {
	ProposalCid string `bson:"proposal_cid"`
	StateID     uint64 `bson:"state_id"`
	StateName   string `bson:"state_name"`
	Miner       string `bson:"miner"`

	PieceCID string `bson:"piece_cid"`
	Size     uint64

	PricePerEpoch uint64 `bson:"price_per_epoch"`
	StartEpoch    uint64 `bson:"start_epoch"`
	Duration      uint64 `bson:"duration"`

	DealID          uint64 `bson:"deal_id"`
	ActivationEpoch int64  `bson:"activation_epoch"`
	Message         string `bson:"message"`
}

// ArchiveConfig is the desired state of a Cid in the Filecoin network.
type ArchiveConfig struct {
	// RepFactor (ignored in Filecoin mainnet) indicates the desired amount of active deals
	// with different miners to store the data. While making deals
	// the other attributes of FilConfig are considered for miner selection.
	RepFactor int `bson:"rep_factor"`
	// DealMinDuration indicates the duration to be used when making new deals.
	DealMinDuration int64 `bson:"deal_min_duration"`
	// ExcludedMiners (ignored in Filecoin mainnet) is a set of miner addresses won't be ever be selected
	// when making new deals, even if they comply to other filters.
	ExcludedMiners []string `bson:"excluded_miners"`
	// TrustedMiners (ignored in Filecoin mainnet) is a set of miner addresses which will be forcibly used
	// when making new deals. An empty/nil list disables this feature.
	TrustedMiners []string `bson:"trusted_miners"`
	// CountryCodes (ignored in Filecoin mainnet) indicates that new deals should select miners on specific
	// countries.
	CountryCodes []string `bson:"country_codes"`
	// Renew indicates deal-renewal configuration.
	Renew ArchiveRenew `bson:"renew"`
	// MaxPrice is the maximum price that will be spent to store the data
	MaxPrice uint64 `bson:"max_price"`
	// FastRetrieval indicates that created deals should enable the
	// fast retrieval feature.
	FastRetrieval bool `bson:"fast_retrieval"`
	// DealStartOffset indicates how many epochs in the future impose a
	// deadline to new deals being active on-chain. This value might influence
	// if miners accept deals, since they should seal fast enough to satisfy
	// this constraint.
	DealStartOffset int64 `bson:"deal_start_offset"`
	// VerifiedDeal indicates that new deals should be marked as verified,
	// assuming the wallet address has enough data-cap.
	VerifiedDeal bool `bson:"verified_deal"`
}

// ArchiveRenew contains renew configuration for a ArchiveConfig.
type ArchiveRenew struct {
	// Enabled indicates that deal-renewal is enabled for this Cid.
	Enabled bool `bson:"enabled"`
	// Threshold indicates how many epochs before expiring should trigger
	// deal renewal. e.g: 100 epoch before expiring.
	Threshold int `bson:"threshold"`
}

type BucketArchives struct {
	col *mongo.Collection
}

func NewBucketArchives(_ context.Context, db *mongo.Database) (*BucketArchives, error) {
	s := &BucketArchives{col: db.Collection("bucketarchives")}
	return s, nil
}

func (k *BucketArchives) Create(ctx context.Context, bucketKey string) (*BucketArchive, error) {
	ba := &BucketArchive{
		BucketKey: bucketKey,
	}
	_, err := k.col.InsertOne(ctx, ba)
	return ba, err
}

func (k *BucketArchives) Replace(ctx context.Context, ba *BucketArchive) error {
	res, err := k.col.ReplaceOne(ctx, bson.M{"_id": ba.BucketKey}, ba)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (k *BucketArchives) GetOrCreate(ctx context.Context, bucketKey string) (*BucketArchive, error) {
	res := k.col.FindOne(ctx, bson.M{"_id": bucketKey})
	if res.Err() != nil {
		if res.Err() == mongo.ErrNoDocuments {
			return k.Create(ctx, bucketKey)
		} else {
			return nil, res.Err()
		}
	}
	var doc BucketArchive
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}
