package rewardstore

import (
	"context"
	"fmt"
	"time"

	"github.com/gogo/status"
	pb "github.com/textileio/textile/v2/api/filrewardsd/pb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	rewardsCollectionName = "filrewards"
)

type reward struct {
	OrgKey            string        `bson:"org_key"`
	DevKey            string        `bson:"dev_key"`
	Type              pb.RewardType `bson:"type"`
	Factor            int64         `bson:"factor"`
	BaseNanoFILReward int64         `bson:"base_nano_fil_reward"`
	CreatedAt         time.Time     `bson:"created_at"`
}

type RewardStore struct {
	col *mongo.Collection
}

func New(db *mongo.Database, debug bool) (*RewardStore, error) {
	col := db.Collection(rewardsCollectionName)

	if _, err := col.Indexes().CreateMany(context.Background(), []mongo.IndexModel{
		{
			Keys:    bson.D{primitive.E{Key: "org_key", Value: 1}, primitive.E{Key: "type", Value: 1}},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"org_key": bson.M{"$gt": ""}}),
		},
		{
			Keys:    bson.D{primitive.E{Key: "dev_key", Value: 1}, primitive.E{Key: "type", Value: 1}},
			Options: options.Index().SetUnique(true).SetPartialFilterExpression(bson.M{"dev_key": bson.M{"$gt": ""}}),
		},
		{
			Keys: bson.D{primitive.E{Key: "org_key", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "dev_key", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "type", Value: 1}},
		},
		{
			Keys: bson.D{primitive.E{Key: "created_at", Value: 1}},
		},
	}); err != nil {
		return nil, fmt.Errorf("creating rewards collection indexes: %v", err)
	}

	return &RewardStore{
		col: col,
	}, nil
}

func (rs *RewardStore) New(ctx context.Context, orgKey, devKey string, t pb.RewardType, factor, baseNanoFilReward int64) (*pb.Reward, error) {
	r := &reward{
		OrgKey:            orgKey,
		DevKey:            devKey,
		Type:              t,
		Factor:            factor,
		BaseNanoFILReward: baseNanoFilReward,
		CreatedAt:         time.Now(),
	}

	if _, err := rs.col.InsertOne(ctx, r); err != nil {
		return nil, fmt.Errorf("inserting reward: %v", err)
	}
	return toPbReward(r), nil
}

func (rs *RewardStore) All(ctx context.Context) ([]*pb.Reward, error) {
	res, err := rs.find(ctx, bson.M{}, options.Find())
	if err != nil {
		return nil, fmt.Errorf("finding all rewards: %v", err)
	}
	return res, nil
}

func (rs *RewardStore) List(ctx context.Context, req *pb.ListRewardsRequest) ([]*pb.Reward, error) {
	findOpts := options.Find()
	findOpts = findOpts.SetLimit(req.PageSize)
	findOpts = findOpts.SetSkip(req.PageSize * req.Page)
	sort := -1
	if req.Ascending {
		sort = 1
	}
	findOpts = findOpts.SetSort(bson.D{primitive.E{Key: "created_at", Value: sort}})
	filter := bson.M{}
	if req.OrgKeyFilter != "" {
		filter["org_key"] = req.OrgKeyFilter
	}
	if req.DevKeyFilter != "" {
		filter["dev_key"] = req.DevKeyFilter
	}
	if req.RewardTypeFilter != pb.RewardType_REWARD_TYPE_UNSPECIFIED {
		filter["type"] = req.RewardTypeFilter
	}

	res, err := rs.find(ctx, filter, findOpts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "finding rewards: %v", err)
	}
	return res, nil
}

func (rs *RewardStore) TotalNanoFilRewarded(ctx context.Context, orgKey string) (int64, error) {
	cursor, err := rs.col.Aggregate(ctx, bson.A{
		bson.M{"$match": bson.M{"org_key": orgKey}},
		bson.M{"$project": bson.M{"amt": bson.M{"$multiply": bson.A{"$factor", "$base_nano_fil_reward"}}}},
		bson.M{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$amt"}}},
	})
	if err != nil {
		return 0, err
	}
	for cursor.Next(ctx) {
		elements, err := cursor.Current.Elements()
		if err != nil {
			return 0, err
		}
		for _, e := range elements {
			if e.Key() == "total" {
				return e.Value().Int64(), nil
			}
		}
	}
	return 0, nil
}

func (rs *RewardStore) find(ctx context.Context, filter bson.M, findOpts *options.FindOptions) ([]*pb.Reward, error) {
	cursor, err := rs.col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var pbRewards []*pb.Reward
	for cursor.Next(ctx) {
		var rec reward
		if err := cursor.Decode(&rec); err != nil {
			return nil, err
		}
		pbRewards = append(pbRewards, toPbReward(&rec))
	}
	if cursor.Err() != nil {
		return nil, cursor.Err()
	}

	return pbRewards, nil
}

func toPbReward(rec *reward) *pb.Reward {
	res := &pb.Reward{
		OrgKey:            rec.OrgKey,
		DevKey:            rec.DevKey,
		Type:              rec.Type,
		Factor:            rec.Factor,
		BaseNanoFilReward: rec.BaseNanoFILReward,
		CreatedAt:         timestamppb.New(rec.CreatedAt),
	}
	return res
}
