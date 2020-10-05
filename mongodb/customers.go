package mongodb

import (
	"context"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	StoredDataUnitSize    = 50 * 1048576  // 50 MiB
	NetworkEgressBinSize  = 100 * 1048576 // 100 MiB
	InstanceReadsBinSize  = 10000
	InstanceWritesBinSize = 5000
)

type Customer struct {
	ID             string         `bson:"_id"`
	Email          string         `bson:"email"`
	SubscriptionID string         `bson:"subscription_id"`
	StoredData     StoredData     `bson:"stored_data"`
	NetworkEgress  NetworkEgress  `bson:"network_egress"`
	InstanceReads  InstanceReads  `bson:"instance_reads"`
	InstanceWrites InstanceWrites `bson:"instance_writes"`
	CreatedAt      int64          `bson:"created_at"`
}

type StoredData struct {
	ItemID    string `bson:"item_id"`
	TotalSize int64  `bson:"total_size"`
	Units     int64  `bson:"units"`
}

type NetworkEgress struct {
	ItemID  string `bson:"item_id"`
	UnitBin int64  `bson:"unit_bin"`
}

type InstanceReads struct {
	ItemID  string `bson:"item_id"`
	UnitBin int64  `bson:"unit_bin"`
}

type InstanceWrites struct {
	ItemID  string `bson:"item_id"`
	UnitBin int64  `bson:"unit_bin"`
}

type Customers struct {
	col *mongo.Collection
}

func NewCustomers(ctx context.Context, db *mongo.Database) (*Customers, error) {
	c := &Customers{col: db.Collection("customers")}
	_, err := c.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"email", 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
	})
	return c, err
}

func (c *Customers) CreateCustomer(ctx context.Context, email, cusID, subID string) (*Customer, error) {
	doc := &Customer{
		ID:             cusID,
		Email:          email,
		SubscriptionID: subID,
		CreatedAt:      time.Now().UnixNano(),
	}
	if _, err := c.col.InsertOne(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (c *Customers) Get(ctx context.Context, id string) (*Customer, error) {
	res := c.col.FindOne(ctx, bson.M{"_id": id})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var doc Customer
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (c *Customers) GetByEmail(ctx context.Context, email string) (*Customer, error) {
	res := c.col.FindOne(ctx, bson.M{"email": email})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var doc Customer
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func (c *Customers) SetStoredDataSize(ctx context.Context, id string, size int64) (units int64, changed bool, err error) {
	sess, err := c.col.Database().Client().StartSession()
	if err != nil {
		return 0, false, err
	}
	defer sess.EndSession(ctx)
	if err = sess.StartTransaction(); err != nil {
		return 0, false, err
	}
	err = mongo.WithSession(ctx, sess, func(sctx mongo.SessionContext) error {
		res := c.col.FindOne(sctx, bson.M{"_id": id})
		if res.Err() != nil {
			return res.Err()
		}
		var doc Customer
		if err := res.Decode(&doc); err != nil {
			return err
		}
		update := bson.M{"stored_data.total_size": size}
		units = int64(math.Round(float64(size) / float64(StoredDataUnitSize)))
		if units != doc.StoredData.Units {
			update["stored_data.units"] = units
			changed = true
		}
		_, err := c.col.UpdateOne(sctx, bson.M{"_id": id}, bson.M{"$set": update})
		if err != nil {
			return err
		}
		return sess.CommitTransaction(sctx)
	})
	return units, changed, err
}

func (c *Customers) IncNetworkEgressSize(ctx context.Context, id string, size int64) (units int64, err error) {
	sess, err := c.col.Database().Client().StartSession()
	if err != nil {
		return 0, err
	}
	defer sess.EndSession(ctx)
	if err = sess.StartTransaction(); err != nil {
		return 0, err
	}
	err = mongo.WithSession(ctx, sess, func(sctx mongo.SessionContext) error {
		res := c.col.FindOneAndUpdate(ctx, bson.M{"_id": id}, bson.M{
			"$inc": bson.M{"network_egress.unit_bin": size},
		}, options.FindOneAndUpdate().SetReturnDocument(options.After))
		if res.Err() != nil {
			return res.Err()
		}
		var doc Customer
		if err := res.Decode(&doc); err != nil {
			return err
		}
		if doc.NetworkEgress.UnitBin >= NetworkEgressBinSize {
			units = doc.NetworkEgress.UnitBin / NetworkEgressBinSize
			binSize := doc.NetworkEgress.UnitBin % NetworkEgressBinSize
			_, err := c.col.UpdateOne(sctx, bson.M{"_id": id}, bson.M{
				"$set": bson.M{"network_egress.unit_bin": binSize},
			})
			if err != nil {
				return err
			}
		}
		return sess.CommitTransaction(sctx)
	})
	return units, err
}

func (c *Customers) IncInstanceReadsCount(ctx context.Context, id string, count int64) (units int64, err error) {
	sess, err := c.col.Database().Client().StartSession()
	if err != nil {
		return 0, err
	}
	defer sess.EndSession(ctx)
	if err = sess.StartTransaction(); err != nil {
		return 0, err
	}
	err = mongo.WithSession(ctx, sess, func(sctx mongo.SessionContext) error {
		res := c.col.FindOneAndUpdate(sctx, bson.M{"_id": id}, bson.M{
			"$inc": bson.M{"instance_reads.unit_bin": count},
		}, options.FindOneAndUpdate().SetReturnDocument(options.After))
		if res.Err() != nil {
			return res.Err()
		}
		var doc Customer
		if err := res.Decode(&doc); err != nil {
			return err
		}
		if doc.InstanceReads.UnitBin >= InstanceReadsBinSize {
			units = doc.InstanceReads.UnitBin / InstanceReadsBinSize
			binCount := doc.InstanceReads.UnitBin % InstanceReadsBinSize
			_, err := c.col.UpdateOne(sctx, bson.M{"_id": id}, bson.M{
				"$set": bson.M{"instance_reads.unit_bin": binCount},
			})
			if err != nil {
				return err
			}
		}
		return sess.CommitTransaction(sctx)
	})
	return units, err
}

func (c *Customers) IncInstanceWritesCount(ctx context.Context, id string, count int64) (units int64, err error) {
	sess, err := c.col.Database().Client().StartSession()
	if err != nil {
		return 0, err
	}
	defer sess.EndSession(ctx)
	if err = sess.StartTransaction(); err != nil {
		return 0, err
	}
	err = mongo.WithSession(ctx, sess, func(sctx mongo.SessionContext) error {
		res := c.col.FindOneAndUpdate(sctx, bson.M{"_id": id}, bson.M{
			"$inc": bson.M{"instance_writes.unit_bin": count},
		}, options.FindOneAndUpdate().SetReturnDocument(options.After))
		if res.Err() != nil {
			return res.Err()
		}
		var doc Customer
		if err := res.Decode(&doc); err != nil {
			return err
		}
		if doc.InstanceWrites.UnitBin >= InstanceWritesBinSize {
			units = doc.InstanceWrites.UnitBin / InstanceWritesBinSize
			binCount := doc.InstanceWrites.UnitBin % InstanceWritesBinSize
			if _, err := c.col.UpdateOne(sctx, bson.M{"_id": id}, bson.M{
				"$set": bson.M{"instance_writes.unit_bin": binCount},
			}); err != nil {
				return err
			}
		}
		return sess.CommitTransaction(sctx)
	})
	return units, err
}

func (c *Customers) Delete(ctx context.Context, id string) error {
	res, err := c.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
