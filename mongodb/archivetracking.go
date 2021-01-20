package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/v2/buckets/archive"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TrackedJob struct {
	JID     string
	Type    archive.TrackedJobType
	ReadyAt time.Time
	Cause   string
	Active  bool

	// Set by TrackedJobTypeArchive type.
	DbID       thread.ID
	DbToken    thread.Token
	BucketKey  string
	BucketRoot cid.Cid
	Owner      thread.PubKey

	// Set by TrackedJobTypeRetrieval type.
	PowToken string
	AccKey   string
}

// trackedJob is an internal representation for storage.
// Any field modifications should be reflected in the cast() func.
type trackedJob struct {
	JID     string                 `bson:"_id"`
	Type    archive.TrackedJobType `bson:"type"`
	ReadyAt time.Time              `bson:"ready_at"`
	Cause   string                 `bson:"cause"`
	Active  bool                   `bson:"active"`

	// Set by TrackedJobTypeArchive type.
	DbID       thread.ID    `bson:"db_id"`
	DbToken    thread.Token `bson:"db_token"`
	BucketKey  string       `bson:"bucket_key"`
	BucketRoot []byte       `bson:"bucket_root"`
	Owner      []byte       `bson:"owner"`

	// Set by TrackedJobTypeRetrieval type.
	PowToken string `bson:"pow_token"`
	AccKey   string
}

type ArchiveTracking struct {
	col *mongo.Collection
}

func NewArchiveTracking(_ context.Context, db *mongo.Database) (*ArchiveTracking, error) {
	s := &ArchiveTracking{
		col: db.Collection("archivetrackings"),
	}
	return s, nil
}

func (at *ArchiveTracking) CreateArchive(ctx context.Context, dbID thread.ID, dbToken thread.Token, bucketKey string, jid string, bucketRoot cid.Cid, owner thread.PubKey) error {
	ownerBytes, err := owner.MarshalBinary()
	if err != nil {
		return fmt.Errorf("marshaling owner to bytes: %v", err)
	}
	newTA := trackedJob{
		Type:       archive.TrackedJobTypeArchive,
		JID:        jid,
		DbID:       dbID,
		DbToken:    dbToken,
		BucketKey:  bucketKey,
		BucketRoot: bucketRoot.Bytes(),
		Owner:      ownerBytes,
		ReadyAt:    time.Now(),
		Cause:      "",
		Active:     true,
	}
	if _, err = at.col.InsertOne(ctx, newTA); err != nil {
		return fmt.Errorf("inserting in collection: %s", err)
	}

	return nil
}

func (at *ArchiveTracking) CreateRetrieval(ctx context.Context, accKey, jobID, powToken string) error {
	newTA := trackedJob{
		Type:     archive.TrackedJobTypeRetrieval,
		JID:      jobID,
		AccKey:   accKey,
		PowToken: powToken,
		ReadyAt:  time.Now(),
		Cause:    "",
		Active:   true,
	}
	if _, err := at.col.InsertOne(ctx, newTA); err != nil {
		return fmt.Errorf("inserting in collection: %s", err)
	}

	return nil
}

func (at *ArchiveTracking) GetReadyToCheck(ctx context.Context, n int64) ([]*TrackedJob, error) {
	opts := options.Find()
	opts.SetLimit(n)
	filter := bson.M{"ready_at": bson.M{"$lte": time.Now()}, "active": true}
	cursor, err := at.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("querying ready tracked archives: %s", err)
	}
	defer cursor.Close(ctx)
	var tas []*trackedJob
	for cursor.Next(ctx) {
		var ta trackedJob
		if err := cursor.Decode(&ta); err != nil {
			return nil, err
		}
		tas = append(tas, &ta)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return castSlice(tas)
}

func (at *ArchiveTracking) Get(ctx context.Context, jid string) (*TrackedJob, error) {
	filter := bson.M{"_id": jid}
	res := at.col.FindOne(ctx, filter)
	if res.Err() != nil {
		return nil, fmt.Errorf("getting tracked archive: %s", res.Err())
	}
	var ta trackedJob
	if err := res.Decode(&ta); err != nil {
		return nil, err
	}
	return cast(&ta)
}

func (at *ArchiveTracking) Finalize(ctx context.Context, jid string, cause string) error {
	res, err := at.col.UpdateOne(ctx, bson.M{"_id": jid}, bson.M{
		"$set": bson.M{
			"active": false,
			"cause":  cause,
		},
	})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (at *ArchiveTracking) Reschedule(ctx context.Context, jid string, dur time.Duration, cause string) error {
	readyAt := time.Now().Add(dur)
	res, err := at.col.UpdateOne(ctx, bson.M{"_id": jid}, bson.M{
		"$set": bson.M{
			"ready_at": readyAt,
			"cause":    cause,
		},
	})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func cast(ta *trackedJob) (*TrackedJob, error) {
	tj := &TrackedJob{
		JID:       ta.JID,
		Type:      ta.Type,
		DbID:      ta.DbID,
		DbToken:   ta.DbToken,
		BucketKey: ta.BucketKey,
		ReadyAt:   ta.ReadyAt,
		Cause:     ta.Cause,
		Active:    ta.Active,
		PowToken:  ta.PowToken,
		AccKey:    ta.AccKey,
	}

	var err error
	if len(ta.BucketRoot) > 0 {
		tj.BucketRoot, err = cid.Cast(ta.BucketRoot)
		if err != nil {
			return nil, fmt.Errorf("casting bucket root: %s", err)
		}
	}
	if len(ta.Owner) > 0 {
		owner := &thread.Libp2pPubKey{}
		err = owner.UnmarshalBinary(ta.Owner)
		if err != nil {
			return nil, fmt.Errorf("unmarshaling public key: %s", err)
		}
		tj.Owner = owner
	}

	return tj, nil
}

func castSlice(tas []*trackedJob) ([]*TrackedJob, error) {
	ret := make([]*TrackedJob, len(tas))
	for i, ta := range tas {
		castedTA, err := cast(ta)
		if err != nil {
			return nil, fmt.Errorf("casting tracked archive slice: %s", err)
		}
		ret[i] = castedTA
	}
	return ret, nil
}
