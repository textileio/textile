package collections

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Org struct {
	Key       crypto.PubKey
	Secret    crypto.PrivKey
	Name      string
	Token     thread.Token
	Members   []Member
	CreatedAt time.Time
}

type Member struct {
	Key      crypto.PubKey
	Username string
	Role     Role
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

func (o *Orgs) Create(ctx context.Context, name string, members []Member) (*Org, error) {
	validName, err := util.ToValidName(name)
	if err != nil {
		return nil, err
	}
	skey, key, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	var haveOwner bool
	for _, m := range members {
		if m.Role == OrgOwner {
			haveOwner = true
			break
		}
	}
	if !haveOwner {
		return nil, fmt.Errorf("an org must have at least one owner")
	}
	doc := &Org{
		Key:       key,
		Secret:    skey,
		Name:      validName,
		Members:   members,
		CreatedAt: time.Now(),
	}
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return nil, err
	}
	secret, err := crypto.MarshalPrivateKey(skey)
	if err != nil {
		return nil, err
	}
	rmems := make(bson.A, len(doc.Members))
	for i, m := range doc.Members {
		k, err := crypto.MarshalPublicKey(m.Key)
		if err != nil {
			return nil, err
		}
		rmems[i] = bson.M{
			"_id":      k,
			"username": m.Username,
			"role":     int(m.Role),
		}
	}
	if _, err = o.col.InsertOne(ctx, bson.M{
		"_id":        id,
		"secret":     secret,
		"name":       doc.Name,
		"members":    rmems,
		"created_at": doc.CreatedAt,
	}); err != nil {
		return nil, err
	}
	return doc, nil
}

func (o *Orgs) Get(ctx context.Context, name string) (*Org, error) {
	res := o.col.FindOne(ctx, bson.M{"name": name})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeOrg(raw)
}

func (o *Orgs) SetToken(ctx context.Context, name string, token thread.Token) error {
	res, err := o.col.UpdateOne(ctx, bson.M{"name": name}, bson.M{"$set": bson.M{"token": token}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (o *Orgs) List(ctx context.Context, member crypto.PubKey) ([]Org, error) {
	mid, err := crypto.MarshalPublicKey(member)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"members": bson.M{"$elemMatch": bson.M{"_id": mid}}}
	cursor, err := o.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var docs []Org
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}
		doc, err := decodeOrg(raw)
		if err != nil {
			return nil, err
		}
		docs = append(docs, *doc)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return docs, nil
}

func (o *Orgs) IsOwner(ctx context.Context, name string, member crypto.PubKey) (bool, error) {
	mid, err := crypto.MarshalPublicKey(member)
	if err != nil {
		return false, err
	}
	filter := bson.M{"name": name, "members": bson.M{"$elemMatch": bson.M{"_id": mid, "role": OrgOwner}}}
	res := o.col.FindOne(ctx, filter)
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return false, nil
		} else {
			return false, res.Err()
		}
	}
	return true, nil
}

func (o *Orgs) IsMember(ctx context.Context, name string, member crypto.PubKey) (bool, error) {
	mid, err := crypto.MarshalPublicKey(member)
	if err != nil {
		return false, err
	}
	filter := bson.M{"name": name, "members": bson.M{"$elemMatch": bson.M{"_id": mid}}}
	res := o.col.FindOne(ctx, filter)
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return false, nil
		} else {
			return false, res.Err()
		}
	}
	return true, nil
}

func (o *Orgs) AddMember(ctx context.Context, name string, member Member) error {
	mk, err := crypto.MarshalPublicKey(member.Key)
	if err != nil {
		return err
	}
	raw := bson.M{
		"_id":      mk,
		"username": member.Username,
		"role":     int(member.Role),
	}
	_, err = o.col.UpdateOne(ctx, bson.M{"name": name, "members._id": bson.M{"$ne": mk}}, bson.M{"$push": bson.M{"members": raw}})
	return err
}

func (o *Orgs) RemoveMember(ctx context.Context, name string, member crypto.PubKey) error {
	isOwner, err := o.IsOwner(ctx, name, member)
	if err != nil {
		return err
	}
	if isOwner { // Ensure there will still be at least one owner left
		cursor, err := o.col.Aggregate(ctx, mongo.Pipeline{
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
	mid, err := crypto.MarshalPublicKey(member)
	if err != nil {
		return err
	}
	res, err := o.col.UpdateOne(ctx, bson.M{"name": name}, bson.M{"$pull": bson.M{"members": bson.M{"_id": mid}}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (o *Orgs) Delete(ctx context.Context, name string) error {
	res, err := o.col.DeleteOne(ctx, bson.M{"name": name})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func decodeOrg(raw bson.M) (*Org, error) {
	skey, err := crypto.UnmarshalPrivateKey(raw["secret"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var token thread.Token
	if v, ok := raw["token"]; ok {
		token = thread.Token(v.(string))
	}
	rmems := raw["members"].(bson.A)
	mems := make([]Member, len(rmems))
	for i, m := range raw["members"].(bson.A) {
		mem := m.(bson.M)
		k, err := crypto.UnmarshalPublicKey(mem["_id"].(primitive.Binary).Data)
		if err != nil {
			return nil, err
		}
		mems[i] = Member{
			Key:      k,
			Username: mem["username"].(string),
			Role:     Role(mem["role"].(int32)),
		}
	}
	var created time.Time
	if v, ok := raw["created_at"]; ok {
		created = v.(primitive.DateTime).Time()
	}
	return &Org{
		Key:       skey.GetPublic(),
		Secret:    skey,
		Name:      raw["name"].(string),
		Token:     token,
		Members:   mems,
		CreatedAt: created,
	}, nil
}
