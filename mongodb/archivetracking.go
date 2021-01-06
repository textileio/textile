package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/ipfs/go-cid"
	"github.com/textileio/go-threads/core/thread"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TrackedArchive struct {
	JID        string
	DbID       thread.ID
	DbToken    thread.Token
	BucketKey  string
	BucketRoot cid.Cid
	Owner      thread.PubKey
	ReadyAt    time.Time
	Cause      string
	Active     bool
}

// trackedArchive is an internal representation for storage.
// Any field modifications should be reflected in the cast() func.
type trackedArchive struct {
	JID        string       `bson:"_id"`
	DbID       thread.ID    `bson:"db_id"`
	DbToken    thread.Token `bson:"db_token"`
	BucketKey  string       `bson:"bucket_key"`
	BucketRoot []byte       `bson:"bucket_root"`
	Owner      []byte       `bson:"owner"`
	ReadyAt    time.Time    `bson:"ready_at"`
	Cause      string       `bson:"cause"`
	Active     bool         `bson:"active"`
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
	newTA := trackedArchive{
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
	_, err = at.col.InsertOne(ctx, newTA)
	if err != nil {
		return fmt.Errorf("inserting in collection: %s", err)
	}
	return nil
}

func (at *ArchiveTracking) CreateRetrieval() error {
	panic("TODO")
}

func (at *ArchiveTracking) GetReadyToCheck(ctx context.Context, n int64) ([]*TrackedArchive, error) {
	opts := options.Find()
	opts.SetLimit(n)
	filter := bson.M{"ready_at": bson.M{"$lte": time.Now()}, "active": true}
	cursor, err := at.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("querying ready tracked archives: %s", err)
	}
	defer cursor.Close(ctx)
	var tas []*trackedArchive
	for cursor.Next(ctx) {
		var ta trackedArchive
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

func (at *ArchiveTracking) Get(ctx context.Context, jid string) (*TrackedArchive, error) {
	filter := bson.M{"_id": jid}
	res := at.col.FindOne(ctx, filter)
	if res.Err() != nil {
		return nil, fmt.Errorf("getting tracked archive: %s", res.Err())
	}
	var ta trackedArchive
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

func cast(ta *trackedArchive) (*TrackedArchive, error) {
	bckCid, err := cid.Cast(ta.BucketRoot)
	if err != nil {
		return nil, fmt.Errorf("casting bucket root: %s", err)
	}
	owner := &thread.Libp2pPubKey{}
	err = owner.UnmarshalBinary(ta.Owner)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling public key: %s", err)
	}
	return &TrackedArchive{
		JID:        ta.JID,
		DbID:       ta.DbID,
		DbToken:    ta.DbToken,
		BucketKey:  ta.BucketKey,
		BucketRoot: bckCid,
		Owner:      owner,
		ReadyAt:    ta.ReadyAt,
		Cause:      ta.Cause,
		Active:     ta.Active,
	}, nil
}

func castSlice(tas []*trackedArchive) ([]*TrackedArchive, error) {
	ret := make([]*TrackedArchive, len(tas))
	for i, ta := range tas {
		castedTA, err := cast(ta)
		if err != nil {
			return nil, fmt.Errorf("casting tracked archive slice: %s", err)
		}
		ret[i] = castedTA
	}
	return ret, nil
}
