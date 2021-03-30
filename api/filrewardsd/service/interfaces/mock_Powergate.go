// Code generated by mockery 2.7.4. DO NOT EDIT.

package interfaces

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	userPb "github.com/textileio/powergate/v2/api/gen/powergate/user/v1"
)

// MockPowergate is an autogenerated mock type for the Powergate type
type MockPowergate struct {
	mock.Mock
}

// Addresses provides a mock function with given fields: ctx
func (_m *MockPowergate) Addresses(ctx context.Context) (*userPb.AddressesResponse, error) {
	ret := _m.Called(ctx)

	var r0 *userPb.AddressesResponse
	if rf, ok := ret.Get(0).(func(context.Context) *userPb.AddressesResponse); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*userPb.AddressesResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
