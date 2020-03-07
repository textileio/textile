package collections

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Org struct {
	ID        primitive.ObjectID `bson:"_id"`
	Name      string             `bson:"name"`
	StoreID   string             `bson:"store_id"`
	Members   []Member           `bson:"members"`
	CreatedAt time.Time          `bson:"created_at"`
}

type Member struct {
	ID       primitive.ObjectID `bson:"_id"`
	Username string             `bson:"username"`
	Role     Role               `bson:"role"`
}

type Role int

const (
	OrgOwner Role = iota
	OrgMember
)

func (r Role) String() (s string) {
	switch r {
	case OrgOwner:
		s = "owner"
	case OrgMember:
		s = "member"
	}
	return
}

func NewOrgContext(ctx context.Context, org *Org) context.Context {
	return context.WithValue(ctx, ctxKey("org"), org)
}

func OrgFromContext(ctx context.Context) (*Org, bool) {
	org, ok := ctx.Value(ctxKey("org")).(*Org)
	return org, ok
}

type Orgs struct {
	col *mongo.Collection
}

func NewOrgs(ctx context.Context, db *mongo.Database) (*Orgs, error) {
	t := &Orgs{col: db.Collection("orgs")}
	_, err := t.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{"name", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{"members._id", 1}},
		},
	})
	return t, err
}

func (t *Orgs) Create(ctx context.Context, doc *Org) error {
	doc.ID = primitive.NewObjectID()
	doc.CreatedAt = time.Now()
	name, err := util.ToValidName(doc.Name)
	if err != nil {
		return err
	}
	doc.Name = name
	if len(doc.Members) == 0 {
		doc.Members = []Member{}
	}
	_, err = t.col.InsertOne(ctx, doc)
	return err
}

func (t *Orgs) Get(ctx context.Context, name string) (*Org, error) {
	var doc *Org
	res := t.col.FindOne(ctx, bson.M{"name": name})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (t *Orgs) List(ctx context.Context, memberID primitive.ObjectID) ([]Org, error) {
	filter := bson.M{"members": bson.M{"$elemMatch": bson.M{"_id": memberID}}}
	cursor, err := t.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var docs []Org
	for cursor.Next(ctx) {
		var doc Org
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return docs, nil
}

func (t *Orgs) IsOwner(ctx context.Context, name string, memberID primitive.ObjectID) (bool, error) {
	filter := bson.M{"name": name, "members": bson.M{"$elemMatch": bson.M{"_id": memberID, "role": OrgOwner}}}
	res := t.col.FindOne(ctx, filter)
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return false, nil
		} else {
			return false, res.Err()
		}
	}
	return true, nil
}

func (t *Orgs) IsMember(ctx context.Context, name string, memberID primitive.ObjectID) (bool, error) {
	filter := bson.M{"name": name, "members": bson.M{"$elemMatch": bson.M{"_id": memberID}}}
	res := t.col.FindOne(ctx, filter)
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return false, nil
		} else {
			return false, res.Err()
		}
	}
	return true, nil
}

func (t *Orgs) AddMember(ctx context.Context, name string, member Member) error {
	res, err := t.col.UpdateOne(ctx, bson.M{"name": name}, bson.M{"$addToSet": bson.M{"members": member}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (t *Orgs) RemoveMember(ctx context.Context, name string, memberID primitive.ObjectID) error {
	isOwner, err := t.IsOwner(ctx, name, memberID)
	if err != nil {
		return err
	}
	if isOwner { // Ensure there will still be at least one owner left
		cursor, err := t.col.Aggregate(ctx, mongo.Pipeline{
			bson.D{{"$match", bson.M{"name": name}}},
			bson.D{{"$project", bson.M{
				"members": bson.M{
					"$filter": bson.M{
						"input": "$members",
						"as":    "member",
						"cond":  bson.M{"$eq": bson.A{"$$member.role", 0}},
					},
				},
			}}},
			bson.D{{"$count", "members"}},
		}, options.Aggregate().SetHint(bson.D{{"name", 1}}))
		if err != nil {
			return err
		}
		type res struct {
			Owners int `bson:"members"`
		}
		var r res
		for cursor.Next(ctx) {
			if err := cursor.Decode(&r); err != nil {
				return err
			}
			break
		}
		if err := cursor.Err(); err != nil {
			return err
		}
		if r.Owners < 2 {
			return fmt.Errorf("an org must have at least one owner")
		}
	}
	res, err := t.col.UpdateOne(ctx, bson.M{"name": name}, bson.M{"$pull": bson.M{"members": bson.M{"_id": memberID}}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (t *Orgs) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := t.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
