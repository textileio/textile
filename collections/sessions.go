package collections

import (
	"time"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	es "github.com/textileio/go-threads/eventstore"
)

var (
	sessionDur = time.Hour * 24 * 7 * 30
)

type Session struct {
	ID     string
	UserID string
	Expiry int
}

type Sessions struct {
	threads *client.Client
	storeID *uuid.UUID
}

func (s *Sessions) GetName() string {
	return "Session"
}

func (s *Sessions) GetInstance() interface{} {
	return &Session{}
}

func (s *Sessions) GetStoreID() *uuid.UUID {
	return s.storeID
}

func (s *Sessions) Create(userID string) (*Session, error) {
	session := &Session{
		UserID: userID,
		Expiry: int(time.Now().Add(sessionDur).Unix()),
	}
	if err := s.threads.ModelCreate(s.storeID.String(), s.GetName(), session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Sessions) Get(id string) (*Session, error) {
	session := &Session{}
	if err := s.threads.ModelFindByID(s.storeID.String(), s.GetName(), id, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Sessions) List(userID string) ([]*Session, error) {
	query := es.JSONWhere("UserID").Eq(userID)
	res, err := s.threads.ModelFind(s.storeID.String(), s.GetName(), query, &Session{})
	if err != nil {
		return nil, err
	}
	return res.([]*Session), nil
}

func (s *Sessions) Touch(session *Session) (err error) {
	session.Expiry = int(time.Now().Add(sessionDur).Unix())
	return s.threads.ModelSave(s.storeID.String(), s.GetName(), session)
}

func (s *Sessions) Delete(id string) error {
	return s.threads.ModelDelete(s.storeID.String(), s.GetName(), id)
}
