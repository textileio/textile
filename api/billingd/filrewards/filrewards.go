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

type Event int

const (
	Unspecified Event = iota
	FirstKeyAccountCreated
	FirstKeyUserCreated // does this fit "register first user?"
	FirstOrgCreated
	InitialBillingSetup
	FirstBucketCreated
	FirstBucketArchiveCreated
	FirstMailboxCreated
	FirstThreadDbCreated
)

const baseAttoFILReward = 1000 // What should this be? Should we read it from mongo?

// maybe we want to read the meta values from mongo so we can update live.
var eventRewards = map[Event]reward{
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
	Event             Event           `bson:"event"`
	Factor            int             `bson:"factor"`
	BaseAttoFILReward int             `bson:"base_atto_fil_reward"`
	CreatedAt         time.Time       `bson:"created_at"`
}

type FilRewards struct {
	col          *mongo.Collection
	rewardsCache map[string]map[Event]struct{}
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
			Keys:    bson.D{primitive.E{Key: "key", Value: 1}, primitive.E{Key: "event", Value: 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		{
			Keys: bson.D{primitive.E{Key: "key", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "event", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "created_at", Value: 1}},
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
	cache := make(map[string]map[Event]struct{})
	for cursor.Next(ctx) {
		var rec RewardRecord
		if err := cursor.Decode(&rec); err != nil {
			return nil, fmt.Errorf("decoding RewardRecord while building cache: %v", err)
		}
		ensureKeyEventCache(cache, rec.Key)
		cache[rec.Key][rec.Event] = struct{}{}
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
		Event:             rewardEvent,
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

type ListRewardRecordsOptions struct {
	KeyFilter   string
	EventFilter Event
	Ascending   bool
	Limit       int
}

func (f *FilRewards) ListRewardRecords(ctx context.Context, opts ListRewardRecordsOptions) ([]RewardRecord, bool, string, error) {
	findOpts := options.Find()
	// ToDo: apply opts to findOpts
	filter := bson.M{}
	cursor, err := f.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, false, "", fmt.Errorf("querying RewardRecords: %v", err)
	}
	defer cursor.Close(ctx)
	var recs []RewardRecord
	for cursor.Next(ctx) {
		var rec RewardRecord
		if err := cursor.Decode(&rec); err != nil {
			return nil, false, "", fmt.Errorf("decoding RewardRecord: %v", err)
		}
		recs = append(recs, rec)
	}
	if err := cursor.Err(); err != nil {
		return nil, false, "", fmt.Errorf("iterating RewardRecords cursor: %v", err)
	}
	return recs, false, "", nil
}

func (f *FilRewards) GetRewardRecord(ctx context.Context, key string, event Event) (*RewardRecord, error) {
	filter := bson.M{"key": key, "event": event}
	res := f.col.FindOne(ctx, filter)
	if res.Err() != nil {
		return nil, fmt.Errorf("getting RewardRecord: %v", res.Err())
	}
	var rec *RewardRecord
	if err := res.Decode(rec); err != nil {
		return nil, fmt.Errorf("decoding RewardRecord: %v", err)
	}
	return rec, nil
}

func ensureKeyEventCache(keyCache map[string]map[Event]struct{}, key string) {
	if _, exists := keyCache[key]; !exists {
		keyCache[key] = map[Event]struct{}{}
	}
}
