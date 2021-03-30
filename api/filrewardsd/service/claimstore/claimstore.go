package claimstore

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/textile/v2/api/filrewardsd/service/interfaces"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	claimsCollectionName = "filclaims"
)

type claim struct {
	ID            primitive.ObjectID `bson:"_id"`
	OrgKey        string             `bson:"org_key"`
	ClaimedBy     string             `bson:"claimed_by"`
	AmountNanoFil int64              `bson:"amount_nano_fil"`
	TxnCid        string             `bson:"txn_cid"`
	CreatedAt     time.Time          `bson:"created_at"`
}

type ClaimStore struct {
	col *mongo.Collection
}

func New(db *mongo.Database, debug bool) (*ClaimStore, error) {
	col := db.Collection(claimsCollectionName)

	if _, err := col.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys: bson.D{primitive.E{Key: "org_key", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "claimed_by", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "created_at", Value: 1}},
		},
	}); err != nil {
		return nil, fmt.Errorf("creating claims collection indexes: %v", err)
	}

	return &ClaimStore{
		col: col,
	}, nil
}

func (cs *ClaimStore) New(ctx context.Context, orgKey, claimedBy string, amountNanoFil int64, msgCid string) (*interfaces.Claim, error) {
	c := &claim{
		ID:            primitive.NewObjectID(),
		OrgKey:        orgKey,
		ClaimedBy:     claimedBy,
		AmountNanoFil: amountNanoFil,
		TxnCid:        msgCid,
		CreatedAt:     time.Now(),
	}
	if _, err := cs.col.InsertOne(ctx, c); err != nil {
		return nil, fmt.Errorf("inserting claim: %v", err)
	}
	return toExternal(c), nil
}

func (cs *ClaimStore) List(ctx context.Context, conf interfaces.ListConfig) ([]*interfaces.Claim, error) {
	findOpts := options.Find()
	findOpts = findOpts.SetLimit(conf.PageSize)
	findOpts = findOpts.SetSkip(conf.PageSize * conf.Page)
	sort := -1
	if conf.Ascending {
		sort = 1
	}
	findOpts = findOpts.SetSort(bson.D{primitive.E{Key: "created_at", Value: sort}})
	filter := bson.M{}
	if conf.OrgKeyFilter != "" {
		filter["org_key"] = conf.OrgKeyFilter
	}
	if conf.ClaimedByFilter != "" {
		filter["claimed_by"] = conf.ClaimedByFilter
	}

	cursor, err := cs.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("querying claims: %v", err)
	}
	defer cursor.Close(ctx)
	var claims []*interfaces.Claim
	for cursor.Next(ctx) {
		var claim claim
		if err := cursor.Decode(&claim); err != nil {
			return nil, err
		}
		claims = append(claims, toExternal(&claim))
	}
	if cursor.Err() != nil {
		return nil, fmt.Errorf("iterating claims cursor: %v", cursor.Err())
	}

	return claims, nil
}

func toExternal(c *claim) *interfaces.Claim {
	return &interfaces.Claim{
		ID:            c.ID.Hex(),
		OrgKey:        c.OrgKey,
		ClaimedBy:     c.ClaimedBy,
		AmountNanoFil: c.AmountNanoFil,
		TxnCid:        c.TxnCid,
		CreatedAt:     c.CreatedAt,
	}
}
