package filrewards

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/textile/v2/api/billingd/analytics"
	mdb "github.com/textileio/textile/v2/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	ErrRecordNotFound       = fmt.Errorf("no RewardRecord found")
	ErrRewardAlreadyClaimed = fmt.Errorf("reward already claimed")
)

const baseAttoFILReward = 1000 // What should this be? Should we read it from mongo?

type Reward int

const (
	Unspecified Reward = iota
	FirstKeyAccountCreated
	FirstKeyUserCreated // does this fit "register first user?"
	FirstOrgCreated
	InitialBillingSetup
	FirstBucketCreated
	FirstBucketArchiveCreated
	FirstMailboxCreated
	FirstThreadDbCreated
)

// maybe we want to read the meta values from mongo so we can update live.
var eventRewards = map[Reward]reward{
	FirstKeyAccountCreated:    {factor: 3},
	FirstKeyUserCreated:       {factor: 1},
	FirstOrgCreated:           {factor: 3},
	InitialBillingSetup:       {factor: 1},
	FirstBucketCreated:        {factor: 2},
	FirstBucketArchiveCreated: {factor: 2},
	FirstMailboxCreated:       {factor: 1},
	FirstThreadDbCreated:      {factor: 1},
}

type reward struct {
	factor int
}

type RewardRecord struct {
	Key               string          `bson:"key"`
	AccountType       mdb.AccountType `bson:"account_type"`
	Reward            Reward          `bson:"reward"`
	Factor            int             `bson:"factor"`
	BaseAttoFILReward int             `bson:"base_atto_fil_reward"`
	CreatedAt         time.Time       `bson:"created_at"`
	ClaimedAt         *time.Time      `bson:"claimed_at"`
}

type FilRewards struct {
	col          *mongo.Collection
	rewardsCache map[string]map[Reward]struct{}
	// ToDo: Need some service to send out notifications when rewards are met.
}

type Config struct {
	DBURI          string
	DBName         string
	CollectionName string
}

func New(ctx context.Context, config Config) (*FilRewards, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(config.DBURI))
	if err != nil {
		return nil, fmt.Errorf("connecting to mongo: %v", err)
	}
	db := client.Database(config.DBName)
	col := db.Collection(config.CollectionName)
	_, err = col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{primitive.E{Key: "key", Value: 1}, primitive.E{Key: "reward", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		{
			Keys: bson.D{primitive.E{Key: "key", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "reward", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "created_at", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "claimed_at", Value: 1}},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("creating indexes: %v", err)
	}
	// Populate cache.
	opts := options.Find()
	cursor, err := col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("querying RewardRecords to populate cache: %s", err)
	}
	defer cursor.Close(ctx)
	cache := make(map[string]map[Reward]struct{})
	for cursor.Next(ctx) {
		var rec RewardRecord
		if err := cursor.Decode(&rec); err != nil {
			return nil, fmt.Errorf("decoding RewardRecord while building cache: %v", err)
		}
		ensureKeyEventCache(cache, rec.Key)
		cache[rec.Key][rec.Reward] = struct{}{}
	}
	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("iterating cursor while building cache: %v", err)
	}
	return &FilRewards{
		col:          col,
		rewardsCache: cache,
	}, nil
}

func (f *FilRewards) ProcessEvent(ctx context.Context, key string, accountType mdb.AccountType, event analytics.Event) (*RewardRecord, error) {
	rewardEvent := Unspecified
	switch event {
	case analytics.KeyAccountCreated:
		rewardEvent = FirstKeyAccountCreated
	case analytics.KeyUserCreated:
		rewardEvent = FirstKeyUserCreated
	case analytics.OrgCreated:
		rewardEvent = FirstOrgCreated
	case analytics.BillingSetup:
		rewardEvent = InitialBillingSetup
	case analytics.BucketCreated:
		rewardEvent = FirstBucketCreated
	case analytics.BucketArchiveCreated:
		rewardEvent = FirstBucketArchiveCreated
	case analytics.MailboxCreated:
		rewardEvent = FirstMailboxCreated
	case analytics.ThreadDbCreated:
		rewardEvent = FirstThreadDbCreated
	}
	if rewardEvent == Unspecified {
		return nil, nil
	}

	ensureKeyEventCache(f.rewardsCache, key)

	if _, exists := f.rewardsCache[key][rewardEvent]; exists {
		// This reward is already granted so bail.
		return nil, nil
	}

	rec := RewardRecord{
		Key:               key,
		AccountType:       accountType,
		Reward:            rewardEvent,
		Factor:            eventRewards[rewardEvent].factor,
		BaseAttoFILReward: baseAttoFILReward,
		CreatedAt:         time.Now(),
	}

	if _, err := f.col.InsertOne(ctx, rec); err != nil {
		return nil, fmt.Errorf("inserting RewardRecord: %v", err)
	}

	f.rewardsCache[key][rewardEvent] = struct{}{}

	// ToDo: Notify the account owner.

	return &rec, nil
}

func (f *FilRewards) ClaimReward(ctx context.Context, key string, reward Reward) error {
	rec, err := f.GetRewardRecord(ctx, key, reward)
	if err != nil {
		return err
	}
	if rec.ClaimedAt != nil {
		return ErrRewardAlreadyClaimed
	}
	filter := bson.M{"key": key, "reward": reward}
	now := time.Now()
	update := bson.M{"$set": bson.M{"claimed_at": &now}}
	res, err := f.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("updating RewardRecord: %v", err)
	}
	if res.ModifiedCount == 0 {
		return fmt.Errorf("modified 0 documents trying to update RewardRecord")
	}
	return nil
}

type ClaimedFilter int

const (
	All ClaimedFilter = iota
	Claimed
	Unclaimed
)

type ListRewardRecordsOptions struct {
	KeyFilter     string
	EventFilter   Reward
	ClaimedFilter ClaimedFilter
	Ascending     bool
	StartAt       *time.Time
	Limit         int64
}

func (f *FilRewards) ListRewardRecords(ctx context.Context, opts ListRewardRecordsOptions) ([]RewardRecord, bool, *time.Time, error) {
	findOpts := options.Find()
	if opts.Limit > 0 {
		findOpts.Limit = &opts.Limit
	}
	sort := -1
	if opts.Ascending {
		sort = 1
	}
	findOpts.Sort = bson.D{primitive.E{Key: "created_at", Value: sort}}
	filter := bson.M{}
	if opts.KeyFilter != "" {
		filter["key"] = opts.KeyFilter
	}
	if opts.EventFilter != Unspecified {
		filter["reward"] = opts.EventFilter
	}
	if opts.ClaimedFilter == Claimed {
		filter["claimed_at"] = bson.M{"$ne": nil}
	}
	if opts.ClaimedFilter == Unclaimed {
		filter["claimed_at"] = bson.M{"$eq": nil}
	}
	comp := "$lt"
	if opts.StartAt != nil {
		if opts.Ascending {
			comp = "$gt"
		}
		filter["created_at"] = bson.M{comp: *opts.StartAt}
	}
	cursor, err := f.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, false, nil, fmt.Errorf("querying RewardRecords: %v", err)
	}
	defer cursor.Close(ctx)
	var recs []RewardRecord
	err = cursor.All(ctx, &recs)
	if err != nil {
		return nil, false, nil, fmt.Errorf("decoding RewardRecord query results: %v", err)
	}

	more := false
	var startAt *time.Time
	if len(recs) > 0 {
		lastCreatedAt := &recs[len(recs)-1].CreatedAt
		filter["created_at"] = bson.M{comp: *lastCreatedAt}
		res := f.col.FindOne(ctx, filter)
		if res.Err() != nil && res.Err() != mongo.ErrNoDocuments {
			return nil, false, nil, fmt.Errorf("checking for more data: %v", err)
		}
		if res.Err() != mongo.ErrNoDocuments {
			more = true
			startAt = lastCreatedAt
		}
	}
	return recs, more, startAt, nil
}

func (f *FilRewards) GetRewardRecord(ctx context.Context, key string, reward Reward) (*RewardRecord, error) {
	filter := bson.M{"key": key, "reward": reward}
	res := f.col.FindOne(ctx, filter)
	if res.Err() == mongo.ErrNoDocuments {
		return nil, ErrRecordNotFound
	}
	if res.Err() != nil {
		return nil, fmt.Errorf("getting RewardRecord: %v", res.Err())
	}
	var rec RewardRecord
	if err := res.Decode(&rec); err != nil {
		return nil, fmt.Errorf("decoding RewardRecord: %v", err)
	}
	return &rec, nil
}

func ensureKeyEventCache(keyCache map[string]map[Reward]struct{}, key string) {
	if _, exists := keyCache[key]; !exists {
		keyCache[key] = map[Reward]struct{}{}
	}
}
