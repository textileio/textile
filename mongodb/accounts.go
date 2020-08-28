package mongodb

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/gosimple/slug"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/textileio/go-threads/core/thread"
	"github.com/textileio/textile/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	usernameRx *regexp.Regexp

	ErrInvalidUsername = fmt.Errorf("username may only contain alphanumeric characters or single hyphens, and cannot begin or end with a hyphen")
)

func init() {
	usernameRx = regexp.MustCompile(`^[a-zA-Z0-9]+(?:-[a-zA-Z0-9]+)?$`)
}

type Account struct {
	Type             AccountType
	Key              crypto.PubKey
	Secret           crypto.PrivKey
	Name             string
	Username         string
	Email            string
	Token            thread.Token
	Members          []Member
	BucketsTotalSize int64
	CreatedAt        time.Time
	PowInfo          *PowInfo
}

type AccountType int

const (
	Dev AccountType = iota
	Org
)

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

func NewDevContext(ctx context.Context, dev *Account) context.Context {
	return context.WithValue(ctx, ctxKey("developer"), dev)
}

func DevFromContext(ctx context.Context) (*Account, bool) {
	dev, ok := ctx.Value(ctxKey("developer")).(*Account)
	return dev, ok
}

func NewOrgContext(ctx context.Context, org *Account) context.Context {
	return context.WithValue(ctx, ctxKey("org"), org)
}

func OrgFromContext(ctx context.Context) (*Account, bool) {
	org, ok := ctx.Value(ctxKey("org")).(*Account)
	return org, ok
}

type Accounts struct {
	col *mongo.Collection
}

func NewAccounts(ctx context.Context, db *mongo.Database) (*Accounts, error) {
	a := &Accounts{col: db.Collection("accounts")}
	_, err := a.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{"username", 1}},
			Options: options.Index().SetUnique(true).
				SetCollation(&options.Collation{Locale: "en", Strength: 2}),
		},
		{
			Keys:    bson.D{{"email", 1}},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		{
			Keys: bson.D{{"members._id", 1}},
		},
	})
	return a, err
}

func (a *Accounts) CreateDev(ctx context.Context, username, email string, powInfo *PowInfo) (*Account, error) {
	if err := a.ValidateUsername(username); err != nil {
		return nil, err
	}
	skey, key, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	doc := &Account{
		Type:      Dev,
		Key:       key,
		Secret:    skey,
		Email:     email,
		Username:  username,
		CreatedAt: time.Now(),
		PowInfo:   powInfo,
	}
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return nil, err
	}
	secret, err := crypto.MarshalPrivateKey(skey)
	if err != nil {
		return nil, err
	}
	data := bson.M{
		"_id":                id,
		"type":               int32(doc.Type),
		"secret":             secret,
		"email":              doc.Email,
		"username":           doc.Username,
		"created_at":         doc.CreatedAt,
		"buckets_total_size": int64(0),
	}
	encodePowInfo(data, doc.PowInfo)
	if _, err := a.col.InsertOne(ctx, data); err != nil {
		return nil, err
	}
	return doc, nil
}

func (a *Accounts) CreateOrg(ctx context.Context, name string, members []Member, powInfo *PowInfo) (*Account, error) {
	slg, ok := util.ToValidName(name)
	if !ok {
		return nil, fmt.Errorf("name '%s' is not available", name)
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
	doc := &Account{
		Type:      Org,
		Key:       key,
		Secret:    skey,
		Name:      name,
		Username:  slg,
		Members:   members,
		CreatedAt: time.Now(),
		PowInfo:   powInfo,
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
			"role":     int32(m.Role),
		}
	}
	data := bson.M{
		"_id":        id,
		"type":       doc.Type,
		"secret":     secret,
		"name":       doc.Name,
		"username":   doc.Username,
		"members":    rmems,
		"created_at": doc.CreatedAt,
	}
	encodePowInfo(data, doc.PowInfo)
	if _, err = a.col.InsertOne(ctx, data); err != nil {
		return nil, err
	}
	return doc, nil
}

func (a *Accounts) UpdatePowInfo(ctx context.Context, key crypto.PubKey, powInfo *PowInfo) (*Account, error) {
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return nil, err
	}
	update := bson.M{}
	encodePowInfo(update, powInfo)
	res, err := a.col.UpdateOne(
		ctx,
		bson.M{"_id": id},
		bson.M{"$set": update},
	)
	if err != nil {
		return nil, err
	}
	if res.ModifiedCount != 1 {
		return nil, fmt.Errorf("should have modified 1 record but updated %v", res.ModifiedCount)
	}
	return a.Get(ctx, key)
}

func (a *Accounts) Get(ctx context.Context, key crypto.PubKey) (*Account, error) {
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return nil, err
	}
	res := a.col.FindOne(ctx, bson.M{"_id": id})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeAccount(raw)
}

func (a *Accounts) GetByUsername(ctx context.Context, username string) (*Account, error) {
	res := a.col.FindOne(ctx, bson.M{"username": username})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeAccount(raw)
}

func (a *Accounts) GetByUsernameOrEmail(ctx context.Context, usernameOrEmail string) (*Account, error) {
	res := a.col.FindOne(ctx, bson.D{{"$or", bson.A{bson.D{{"username", usernameOrEmail}}, bson.D{{"email", usernameOrEmail}}}}})
	if res.Err() != nil {
		return nil, res.Err()
	}
	var raw bson.M
	if err := res.Decode(&raw); err != nil {
		return nil, err
	}
	return decodeAccount(raw)
}

func (a *Accounts) ValidateUsername(username string) error {
	if !usernameRx.MatchString(username) {
		return ErrInvalidUsername
	}
	return nil
}

func (a *Accounts) IsUsernameAvailable(ctx context.Context, username string) error {
	if err := a.ValidateUsername(username); err != nil {
		return err
	}
	res := a.col.FindOne(ctx, bson.M{"username": username})
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return nil
		}
		return res.Err()
	}
	return fmt.Errorf("username '%s' is not available", username)
}

func (a *Accounts) IsNameAvailable(ctx context.Context, name string) (s string, err error) {
	s = slug.Make(name)
	res := a.col.FindOne(ctx, bson.M{"username": s})
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return
		}
		err = res.Err()
		return
	}
	return s, fmt.Errorf("the name '%s' is already taken", name)
}

func (a *Accounts) SetToken(ctx context.Context, key crypto.PubKey, token thread.Token) error {
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return err
	}
	res, err := a.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"token": token}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (a *Accounts) SetBucketsTotalSize(ctx context.Context, key crypto.PubKey, newTotalSize int64) error {
	if newTotalSize < 0 {
		return fmt.Errorf("new size %d must be positive", newTotalSize)
	}
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return err
	}
	res, err := a.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"buckets_total_size": newTotalSize}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (a *Accounts) ListByMember(ctx context.Context, member crypto.PubKey) ([]Account, error) {
	mid, err := crypto.MarshalPublicKey(member)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"members": bson.M{"$elemMatch": bson.M{"_id": mid}}}
	cursor, err := a.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []Account
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}
		doc, err := decodeAccount(raw)
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

func (a *Accounts) ListByOwner(ctx context.Context, owner crypto.PubKey) ([]Account, error) {
	oid, err := crypto.MarshalPublicKey(owner)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"members": bson.M{"$elemMatch": bson.M{"_id": oid, "role": OrgOwner}}}
	cursor, err := a.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []Account
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}
		doc, err := decodeAccount(raw)
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

func (a *Accounts) ListMembers(ctx context.Context, members []Member) ([]Account, error) {
	keys := make([][]byte, len(members))
	var err error
	for i, m := range members {
		keys[i], err = crypto.MarshalPublicKey(m.Key)
		if err != nil {
			return nil, err
		}
	}
	cursor, err := a.col.Find(ctx, bson.M{"_id": bson.M{"$in": keys}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var docs []Account
	for cursor.Next(ctx) {
		var raw bson.M
		if err := cursor.Decode(&raw); err != nil {
			return nil, err
		}
		doc, err := decodeAccount(raw)
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

func (a *Accounts) IsOwner(ctx context.Context, username string, member crypto.PubKey) (bool, error) {
	mid, err := crypto.MarshalPublicKey(member)
	if err != nil {
		return false, err
	}
	filter := bson.M{"username": username, "members": bson.M{"$elemMatch": bson.M{"_id": mid, "role": OrgOwner}}}
	res := a.col.FindOne(ctx, filter)
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return false, nil
		} else {
			return false, res.Err()
		}
	}
	return true, nil
}

func (a *Accounts) IsMember(ctx context.Context, username string, member crypto.PubKey) (bool, error) {
	mid, err := crypto.MarshalPublicKey(member)
	if err != nil {
		return false, err
	}
	filter := bson.M{"username": username, "members": bson.M{"$elemMatch": bson.M{"_id": mid}}}
	res := a.col.FindOne(ctx, filter)
	if res.Err() != nil {
		if errors.Is(res.Err(), mongo.ErrNoDocuments) {
			return false, nil
		} else {
			return false, res.Err()
		}
	}
	return true, nil
}

func (a *Accounts) AddMember(ctx context.Context, username string, member Member) error {
	mk, err := crypto.MarshalPublicKey(member.Key)
	if err != nil {
		return err
	}
	raw := bson.M{
		"_id":      mk,
		"username": member.Username,
		"role":     int(member.Role),
	}
	_, err = a.col.UpdateOne(ctx, bson.M{"username": username, "members._id": bson.M{"$ne": mk}}, bson.M{"$push": bson.M{"members": raw}})
	return err
}

func (a *Accounts) RemoveMember(ctx context.Context, username string, member crypto.PubKey) error {
	isOwner, err := a.IsOwner(ctx, username, member)
	if err != nil {
		return err
	}
	if isOwner { // Ensure there will still be at least one owner left
		cursor, err := a.col.Aggregate(ctx, mongo.Pipeline{
			bson.D{{"$match", bson.M{"username": username}}},
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
		}, options.Aggregate().SetHint(bson.D{{"username", 1}}))
		if err != nil {
			return err
		}
		defer cursor.Close(ctx)
		type res struct {
			Owners int `bson:"members"`
		}
		var r res
		cursor.Next(ctx)
		if err := cursor.Decode(&r); err != nil {
			return err
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
	res, err := a.col.UpdateOne(ctx, bson.M{"username": username}, bson.M{"$pull": bson.M{"members": bson.M{"_id": mid}}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (a *Accounts) Delete(ctx context.Context, key crypto.PubKey) error {
	id, err := crypto.MarshalPublicKey(key)
	if err != nil {
		return err
	}
	res, err := a.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func decodeAccount(raw bson.M) (*Account, error) {
	var name, email string
	if v, ok := raw["name"]; ok {
		name = v.(string)
	}
	if v, ok := raw["email"]; ok {
		email = v.(string)
	}
	var totalSize int64
	if v, ok := raw["buckets_total_size"]; ok {
		totalSize = v.(int64)
	}
	skey, err := crypto.UnmarshalPrivateKey(raw["secret"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var token thread.Token
	if v, ok := raw["token"]; ok {
		token = thread.Token(v.(string))
	}
	var mems []Member
	if v, ok := raw["members"]; ok {
		rmems := v.(bson.A)
		mems = make([]Member, len(rmems))

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
	}
	var created time.Time
	if v, ok := raw["created_at"]; ok {
		created = v.(primitive.DateTime).Time()
	}
	return &Account{
		Type:             AccountType(raw["type"].(int32)),
		Key:              skey.GetPublic(),
		Secret:           skey,
		Name:             name,
		Username:         raw["username"].(string),
		Email:            email,
		Token:            token,
		Members:          mems,
		BucketsTotalSize: totalSize,
		CreatedAt:        created,
		PowInfo:          decodePowInfo(raw),
	}, nil
}
