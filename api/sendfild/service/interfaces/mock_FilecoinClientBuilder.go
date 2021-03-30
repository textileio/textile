// Code generated by mockery 2.7.4. DO NOT EDIT.

package interfaces

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockFilecoinClientBuilder is an autogenerated mock type for the FilecoinClientBuilder type
type MockFilecoinClientBuilder struct {
	mock.Mock
}

// Execute provides a mock function with given fields: _a0
func (_m *MockFilecoinClientBuilder) Execute(_a0 context.Context) (FilecoinClient, func(), error) {
	ret := _m.Called(_a0)

	var r0 FilecoinClient
	if rf, ok := ret.Get(0).(func(context.Context) FilecoinClient); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(FilecoinClient)
		}
	}

	var r1 func()
	if rf, ok := ret.Get(1).(func(context.Context) func()); ok {
		r1 = rf(_a0)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(func())
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(context.Context) error); ok {
		r2 = rf(_a0)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}
