// Code generated by mockery 2.7.4. DO NOT EDIT.

package interfaces

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	mongodb "github.com/textileio/textile/v2/mongodb"

	thread "github.com/textileio/go-threads/core/thread"
)

// MockAccountStore is an autogenerated mock type for the AccountStore type
type MockAccountStore struct {
	mock.Mock
}

// Get provides a mock function with given fields: ctx, key
func (_m *MockAccountStore) Get(ctx context.Context, key thread.PubKey) (*mongodb.Account, error) {
	ret := _m.Called(ctx, key)

	var r0 *mongodb.Account
	if rf, ok := ret.Get(0).(func(context.Context, thread.PubKey) *mongodb.Account); ok {
		r0 = rf(ctx, key)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*mongodb.Account)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, thread.PubKey) error); ok {
		r1 = rf(ctx, key)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
