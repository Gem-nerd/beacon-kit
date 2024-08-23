// Code generated by mockery v2.45.0. DO NOT EDIT.

package mocks

import (
	service "github.com/berachain/beacon-kit/mod/node-core/pkg/services/registry"
	mock "github.com/stretchr/testify/mock"
)

// RegistryOption is an autogenerated mock type for the RegistryOption type
type RegistryOption struct {
	mock.Mock
}

type RegistryOption_Expecter struct {
	mock *mock.Mock
}

func (_m *RegistryOption) EXPECT() *RegistryOption_Expecter {
	return &RegistryOption_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: _a0
func (_m *RegistryOption) Execute(_a0 *service.Registry) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*service.Registry) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RegistryOption_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type RegistryOption_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - _a0 *service.Registry
func (_e *RegistryOption_Expecter) Execute(_a0 interface{}) *RegistryOption_Execute_Call {
	return &RegistryOption_Execute_Call{Call: _e.mock.On("Execute", _a0)}
}

func (_c *RegistryOption_Execute_Call) Run(run func(_a0 *service.Registry)) *RegistryOption_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*service.Registry))
	})
	return _c
}

func (_c *RegistryOption_Execute_Call) Return(_a0 error) *RegistryOption_Execute_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *RegistryOption_Execute_Call) RunAndReturn(run func(*service.Registry) error) *RegistryOption_Execute_Call {
	_c.Call.Return(run)
	return _c
}

// NewRegistryOption creates a new instance of RegistryOption. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewRegistryOption(t interface {
	mock.TestingT
	Cleanup(func())
}) *RegistryOption {
	mock := &RegistryOption{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
