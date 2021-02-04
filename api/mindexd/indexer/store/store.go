package store

import "go.mongodb.org/mongo-driver/mongo"

type Store struct {
	db *mongo.Database
}

func New(db *mongo.Database) (*Store, error) {
	s := &Store{
		db: db,
	}
	return s, nil
}
