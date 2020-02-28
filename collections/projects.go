package collections

import (
	"context"
	"time"

	"github.com/textileio/go-threads/api/client"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Project struct {
	ID        primitive.ObjectID   `bson:"_id"`
	OwnerID   primitive.ObjectID   `bson:"owner_id"`
	Name      string               `bson:"name"`
	StoreID   string               `bson:"store_id"`
	Address   string               `bson:"address"`
	Members   []primitive.ObjectID `bson:"members"`
	Teams     []primitive.ObjectID `bson:"teams"`
	CreatedAt time.Time            `bson:"created_at"`
}

type Projects struct {
	col     *mongo.Collection
	threads *client.Client
	token   string
}

func NewProjects(ctx context.Context, db *mongo.Database) (*Projects, error) {
	p := &Projects{col: db.Collection("projects")}
	_, err := p.col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{"owner_id", 1}, {"name", 1}},
		},
		{
			Keys:    bson.D{{"members", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{"teams", 1}},
			Options: options.Index().SetUnique(true),
		},
	})
	return p, err
}

func (p *Projects) Create(
	ctx context.Context, ownerID primitive.ObjectID, name, storeID, addr string) (*Project, error) {
	validName, err := toValidName(name)
	if err != nil {
		return nil, err
	}
	doc := &Project{
		OwnerID:   ownerID,
		Name:      validName,
		StoreID:   storeID,
		Address:   addr,
		CreatedAt: time.Now(),
	}
	res, err := p.col.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}
	doc.ID = res.InsertedID.(primitive.ObjectID)
	return doc, nil
}

func (p *Projects) Get(ctx context.Context, id primitive.ObjectID) (*Project, error) {
	var doc *Project
	res := p.col.FindOne(ctx, bson.M{"_id": id})
	if res.Err() != nil {
		return nil, res.Err()
	}
	if err := res.Decode(&doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func (p *Projects) List(ctx context.Context, developerID primitive.ObjectID) ([]Project, error) {
	filter := bson.M{"$or": bson.A{
		bson.M{"members": bson.M{"$elemMatch": bson.M{"$eq": developerID}}},
	}}
	cursor, err := p.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var docs []Project
	for cursor.Next(ctx) {
		var doc Project
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

// @todo: Remove associated thread store
func (p *Projects) Delete(ctx context.Context, id primitive.ObjectID) error {
	res, err := p.col.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
