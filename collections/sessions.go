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

func (s *Sessions) SetThreads(threads *client.Client) {
	s.threads = threads
}

func (s *Sessions) GetStoreID() *uuid.UUID {
	return s.storeID
}

func (s *Sessions) SetStoreID(id *uuid.UUID) {
	s.storeID = id
}

func (s *Sessions) Create(session *Session) error {
	return s.threads.ModelCreate(s.storeID.String(), s.GetName(), session)
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

func (s *Sessions) Update(session *Session) error {
	return s.threads.ModelSave(s.storeID.String(), s.GetName(), session)
}

func (s *Sessions) Delete(id string) error {
	return s.threads.ModelDelete(s.storeID.String(), s.GetName(), id)
}

func (s *Sessions) Touch(id string) (err error) {
	txn, err := s.threads.WriteTransaction(s.storeID.String(), s.GetName())
	if err != nil {
		return
	}
	done, err := txn.Start()
	if err != nil {
		return
	}
	defer func() {
		doneErr := done()
		if err == nil {
			err = doneErr
		}
	}()
	session := &Session{}
	if err = txn.FindByID(id, session); err != nil {
		return
	}
	session.Expiry = int(time.Now().Add(sessionDur).Unix())
	return txn.Save(session)
}
