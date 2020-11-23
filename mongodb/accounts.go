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
	"github.com/textileio/textile/v2/util"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	usernameRx *regexp.Regexp

	ErrInvalidUsername = fmt.Errorf("username may only contain alphanumeric characters or single hyphens, " +
		"and cannot begin or end with a hyphen")
)

func init() {
	usernameRx = regexp.MustCompile(`^[a-zA-Z0-9]+(?:-[a-zA-Z0-9]+)?$`)
}

type Account struct {
	Type      AccountType
	Key       thread.PubKey
	Secret    thread.Identity
	Name      string
	Username  string
	Email     string
	Token     thread.Token
	Members   []Member
	PowInfo   *PowInfo
	CreatedAt time.Time
}

type AccountType int

const (
	Dev AccountType = iota
	Org
	User
)

type Member struct {
	Key      thread.PubKey
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

type AccountCtx struct {
	User *Account
	Org  *Account
}

func NewAccountContext(ctx context.Context, user, org *Account) context.Context {
	return context.WithValue(ctx, ctxKey("account"), &AccountCtx{
		User: user,
		Org:  org,
	})
}

func AccountCtxForAccount(account *Account) *AccountCtx {
	actx := &AccountCtx{}
	if account.Type == Org {
		actx.Org = account
	} else {
		actx.User = account
	}
	return actx
}

func AccountFromContext(ctx context.Context) (*AccountCtx, bool) {
	acc, ok := ctx.Value(ctxKey("account")).(*AccountCtx)
	return acc, ok
}

func (ac *AccountCtx) Owner() *Account {
	if ac.Org != nil {
		return ac.Org
	}
	return ac.User
}

type Accounts struct {
	col *mongo.Collection
}

func NewAccounts(ctx context.Context, db *mongo.Database) (*Accounts, error) {
	a := &Accounts{col: db.Collection("accounts")}
	_, err := a.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{primitive.E{Key: "username", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetCollation(&options.Collation{Locale: "en", Strength: 2}).
				SetSparse(true),
		},
		{
			Keys: bson.D{primitive.E{Key: "email", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetSparse(true),
		},
		{
			Keys: bson.D{primitive.E{Key: "members._id", Value: 1}},
		},
	})
	return a, err
}

func (a *Accounts) CreateDev(ctx context.Context, username, email string, powInfo *PowInfo) (*Account, error) {
	if err := a.ValidateUsername(username); err != nil {
		return nil, err
	}
	sk, pk, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, err
	}
	doc := &Account{
		Type:      Dev,
		Key:       thread.NewLibp2pPubKey(pk),
		Secret:    thread.NewLibp2pIdentity(sk),
		Email:     email,
		Username:  username,
		PowInfo:   powInfo,
		CreatedAt: time.Now(),
	}
	id, err := doc.Key.MarshalBinary()
	if err != nil {
		return nil, err
	}
	secret, err := doc.Secret.MarshalBinary()
	if err != nil {
		return nil, err
	}
	data := bson.M{
		"_id":        id,
		"type":       int32(doc.Type),
		"secret":     secret,
		"email":      doc.Email,
		"username":   doc.Username,
		"created_at": doc.CreatedAt,
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
	sk, pk, err := crypto.GenerateEd25519Key(rand.Reader)
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
		Key:       thread.NewLibp2pPubKey(pk),
		Secret:    thread.NewLibp2pIdentity(sk),
		Name:      name,
		Username:  slg,
		Members:   members,
		PowInfo:   powInfo,
		CreatedAt: time.Now(),
	}
	id, err := doc.Key.MarshalBinary()
	if err != nil {
		return nil, err
	}
	secret, err := doc.Secret.MarshalBinary()
	if err != nil {
		return nil, err
	}
	rmems := make(bson.A, len(doc.Members))
	for i, m := range doc.Members {
		k, err := m.Key.MarshalBinary()
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

func (a *Accounts) CreateUser(ctx context.Context, key thread.PubKey, powInfo *PowInfo) (*Account, error) {
	doc := &Account{
		Type:      User,
		Key:       key,
		PowInfo:   powInfo,
		CreatedAt: time.Now(),
	}
	id, err := doc.Key.MarshalBinary()
	if err != nil {
		return nil, err
	}
	data := bson.M{
		"_id":        id,
		"type":       int32(doc.Type),
		"created_at": doc.CreatedAt,
	}
	encodePowInfo(data, doc.PowInfo)
	if _, err := a.col.InsertOne(ctx, data); err != nil {
		return nil, err
	}
	return doc, nil
}

func (a *Accounts) Get(ctx context.Context, key thread.PubKey) (*Account, error) {
	id, err := key.MarshalBinary()
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
	res := a.col.FindOne(ctx, bson.D{
		primitive.E{Key: "$or", Value: bson.A{
			bson.D{
				primitive.E{Key: "username", Value: usernameOrEmail},
			},
			bson.D{
				primitive.E{Key: "email", Value: usernameOrEmail},
			},
		}},
	})
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

func (a *Accounts) SetToken(ctx context.Context, key thread.PubKey, token thread.Token) error {
	id, err := key.MarshalBinary()
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

func (a *Accounts) UpdatePowInfo(ctx context.Context, key thread.PubKey, powInfo *PowInfo) (*Account, error) {
	id, err := key.MarshalBinary()
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

func (a *Accounts) ListByMember(ctx context.Context, member thread.PubKey) ([]Account, error) {
	mid, err := member.MarshalBinary()
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

func (a *Accounts) ListByOwner(ctx context.Context, owner thread.PubKey) ([]Account, error) {
	oid, err := owner.MarshalBinary()
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
		keys[i], err = m.Key.MarshalBinary()
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

func (a *Accounts) IsOwner(ctx context.Context, username string, member thread.PubKey) (bool, error) {
	mid, err := member.MarshalBinary()
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

func (a *Accounts) IsMember(ctx context.Context, username string, member thread.PubKey) (bool, error) {
	mid, err := member.MarshalBinary()
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
	mk, err := member.Key.MarshalBinary()
	if err != nil {
		return err
	}
	raw := bson.M{
		"_id":      mk,
		"username": member.Username,
		"role":     int(member.Role),
	}
	_, err = a.col.UpdateOne(ctx, bson.M{
		"username":    username,
		"members._id": bson.M{"$ne": mk},
	}, bson.M{"$push": bson.M{"members": raw}})
	return err
}

func (a *Accounts) RemoveMember(ctx context.Context, username string, member thread.PubKey) error {
	isOwner, err := a.IsOwner(ctx, username, member)
	if err != nil {
		return err
	}
	if isOwner { // Ensure there will still be at least one owner left
		cursor, err := a.col.Aggregate(ctx, mongo.Pipeline{
			bson.D{primitive.E{Key: "$match", Value: bson.M{"username": username}}},
			bson.D{primitive.E{Key: "$project", Value: bson.M{
				"members": bson.M{
					"$filter": bson.M{
						"input": "$members",
						"as":    "member",
						"cond":  bson.M{"$eq": bson.A{"$$member.role", 0}},
					},
				},
			}}},
			bson.D{primitive.E{Key: "$count", Value: "members"}},
		}, options.Aggregate().SetHint(bson.D{primitive.E{Key: "username", Value: 1}}))
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
	mid, err := member.MarshalBinary()
	if err != nil {
		return err
	}
	res, err := a.col.UpdateOne(ctx, bson.M{
		"username": username,
	}, bson.M{"$pull": bson.M{"members": bson.M{"_id": mid}}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (a *Accounts) Delete(ctx context.Context, key thread.PubKey) error {
	id, err := key.MarshalBinary()
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
	key := &thread.Libp2pPubKey{}
	err := key.UnmarshalBinary(raw["_id"].(primitive.Binary).Data)
	if err != nil {
		return nil, err
	}
	var username, name, email string
	if v, ok := raw["username"]; ok {
		username = v.(string)
	}
	if v, ok := raw["name"]; ok {
		name = v.(string)
	}
	if v, ok := raw["email"]; ok {
		email = v.(string)
	}
	var secret *thread.Libp2pIdentity
	if v, ok := raw["secret"]; ok {
		secret = &thread.Libp2pIdentity{}
		err := secret.UnmarshalBinary(v.(primitive.Binary).Data)
		if err != nil {
			return nil, err
		}
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
			k := &thread.Libp2pPubKey{}
			err := k.UnmarshalBinary(mem["_id"].(primitive.Binary).Data)
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
		Type:      AccountType(raw["type"].(int32)),
		Key:       key,
		Secret:    secret,
		Name:      name,
		Username:  username,
		Email:     email,
		Token:     token,
		Members:   mems,
		PowInfo:   decodePowInfo(raw),
		CreatedAt: created,
	}, nil
}
