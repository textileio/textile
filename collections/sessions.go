package collections

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/textileio/go-threads/api/client"
	"github.com/textileio/go-threads/store"
)

var (
	sessionDur = time.Hour * 24 * 7 * 30
)

type Session struct {
	ID     string
	UserID string // user or app user
	Scope  string // user or team ID
	Expiry int
}

type Sessions struct {
	threads *client.Client
	storeID *uuid.UUID
	token   string
}

func (s *Sessions) GetName() string {
	return "Session"
}

func (s *Sessions) GetInstance() interface{} {
	return &Session{}
}

func (s *Sessions) GetIndexes() []*store.IndexConfig {
	return []*store.IndexConfig{}
}

func (s *Sessions) GetStoreID() *uuid.UUID {
	return s.storeID
}

func (s *Sessions) Create(ctx context.Context, userID, scope string) (*Session, error) {
	ctx = AuthCtx(ctx, s.token)
	session := &Session{
		UserID: userID,
		Scope:  scope,
		Expiry: int(time.Now().Add(sessionDur).Unix()),
	}
	if err := s.threads.ModelCreate(ctx, s.storeID.String(), s.GetName(), session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Sessions) Get(ctx context.Context, id string) (*Session, error) {
	ctx = AuthCtx(ctx, s.token)
	session := &Session{}
	if err := s.threads.ModelFindByID(ctx, s.storeID.String(), s.GetName(), id, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *Sessions) Save(ctx context.Context, session *Session) error {
	ctx = AuthCtx(ctx, s.token)
	return s.threads.ModelSave(ctx, s.storeID.String(), s.GetName(), session)
}

func (s *Sessions) Touch(ctx context.Context, session *Session) error {
	ctx = AuthCtx(ctx, s.token)
	session.Expiry = int(time.Now().Add(sessionDur).Unix())
	return s.threads.ModelSave(ctx, s.storeID.String(), s.GetName(), session)
}

func (s *Sessions) Delete(ctx context.Context, id string) error {
	ctx = AuthCtx(ctx, s.token)
	return s.threads.ModelDelete(ctx, s.storeID.String(), s.GetName(), id)
}
