// Code generated by mockery 2.7.4. DO NOT EDIT.

package interfaces

import (
	api "github.com/filecoin-project/lotus/api"
	cid "github.com/ipfs/go-cid"

	context "context"

	mock "github.com/stretchr/testify/mock"

	types "github.com/filecoin-project/lotus/chain/types"
)

// MockFilecoinClient is an autogenerated mock type for the FilecoinClient type
type MockFilecoinClient struct {
	mock.Mock
}

// MpoolPushMessage provides a mock function with given fields: ctx, msg, spec
func (_m *MockFilecoinClient) MpoolPushMessage(ctx context.Context, msg *types.Message, spec *api.MessageSendSpec) (*types.SignedMessage, error) {
	ret := _m.Called(ctx, msg, spec)

	var r0 *types.SignedMessage
	if rf, ok := ret.Get(0).(func(context.Context, *types.Message, *api.MessageSendSpec) *types.SignedMessage); ok {
		r0 = rf(ctx, msg, spec)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.SignedMessage)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *types.Message, *api.MessageSendSpec) error); ok {
		r1 = rf(ctx, msg, spec)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// StateWaitMsg provides a mock function with given fields: ctx, msgc, confidence
func (_m *MockFilecoinClient) StateWaitMsg(ctx context.Context, msgc cid.Cid, confidence uint64) (*api.MsgLookup, error) {
	ret := _m.Called(ctx, msgc, confidence)

	var r0 *api.MsgLookup
	if rf, ok := ret.Get(0).(func(context.Context, cid.Cid, uint64) *api.MsgLookup); ok {
		r0 = rf(ctx, msgc, confidence)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*api.MsgLookup)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, cid.Cid, uint64) error); ok {
		r1 = rf(ctx, msgc, confidence)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
