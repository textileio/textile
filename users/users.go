package users

import "github.com/google/uuid"

type User struct {
	ID uuid.UUID
}

func NewUser() (*User, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	return &User{ID: id}, nil
}
