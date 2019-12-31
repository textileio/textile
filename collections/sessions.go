package collections

import (
	"time"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
)

var (
	sessionDur = time.Hour * 24 * 7 * 30
)

type Session struct {
	ID     string
	UserID string
	Scope  string // user or team ID
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

func (s *Sessions) Create(userID, scope string) (*Session, error) {
	session := &Session{
		UserID: userID,
		Scope:  scope,
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

func (s *Sessions) Touch(session *Session) error {
	session.Expiry = int(time.Now().Add(sessionDur).Unix())
	return s.threads.ModelSave(s.storeID.String(), s.GetName(), session)
}

func (s *Sessions) SwitchScope(session *Session, scope string) error {
	session.Scope = scope
	return s.threads.ModelSave(s.storeID.String(), s.GetName(), session)
}

func (s *Sessions) Delete(id string) error {
	return s.threads.ModelDelete(s.storeID.String(), s.GetName(), id)
}
