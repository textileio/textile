// Code generated by mockery 2.7.4. DO NOT EDIT.

package interfaces

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockClaimStore is an autogenerated mock type for the ClaimStore type
type MockClaimStore struct {
	mock.Mock
}

// List provides a mock function with given fields: ctx, conf
func (_m *MockClaimStore) List(ctx context.Context, conf ListConfig) ([]*Claim, error) {
	ret := _m.Called(ctx, conf)

	var r0 []*Claim
	if rf, ok := ret.Get(0).(func(context.Context, ListConfig) []*Claim); ok {
		r0 = rf(ctx, conf)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*Claim)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, ListConfig) error); ok {
		r1 = rf(ctx, conf)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// New provides a mock function with given fields: ctx, orgKey, claimedBy, amountNanoFil, msgCid
func (_m *MockClaimStore) New(ctx context.Context, orgKey string, claimedBy string, amountNanoFil int64, msgCid string) (*Claim, error) {
	ret := _m.Called(ctx, orgKey, claimedBy, amountNanoFil, msgCid)

	var r0 *Claim
	if rf, ok := ret.Get(0).(func(context.Context, string, string, int64, string) *Claim); ok {
		r0 = rf(ctx, orgKey, claimedBy, amountNanoFil, msgCid)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*Claim)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, int64, string) error); ok {
		r1 = rf(ctx, orgKey, claimedBy, amountNanoFil, msgCid)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
