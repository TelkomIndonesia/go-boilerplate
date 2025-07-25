// Code generated by mockery v2.53.4. DO NOT EDIT.

package profilemock

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
	profile "github.com/telkomindonesia/go-boilerplate/internal/profile"

	uuid "github.com/google/uuid"
)

// MockProfileRepository is an autogenerated mock type for the ProfileRepository type
type MockProfileRepository struct {
	mock.Mock
}

type MockProfileRepository_Expecter struct {
	mock *mock.Mock
}

func (_m *MockProfileRepository) EXPECT() *MockProfileRepository_Expecter {
	return &MockProfileRepository_Expecter{mock: &_m.Mock}
}

// FetchProfile provides a mock function with given fields: ctx, tenantID, id
func (_m *MockProfileRepository) FetchProfile(ctx context.Context, tenantID uuid.UUID, id uuid.UUID) (*profile.Profile, error) {
	ret := _m.Called(ctx, tenantID, id)

	if len(ret) == 0 {
		panic("no return value specified for FetchProfile")
	}

	var r0 *profile.Profile
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, uuid.UUID) (*profile.Profile, error)); ok {
		return rf(ctx, tenantID, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, uuid.UUID) *profile.Profile); ok {
		r0 = rf(ctx, tenantID, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*profile.Profile)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID, uuid.UUID) error); ok {
		r1 = rf(ctx, tenantID, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockProfileRepository_FetchProfile_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FetchProfile'
type MockProfileRepository_FetchProfile_Call struct {
	*mock.Call
}

// FetchProfile is a helper method to define mock.On call
//   - ctx context.Context
//   - tenantID uuid.UUID
//   - id uuid.UUID
func (_e *MockProfileRepository_Expecter) FetchProfile(ctx interface{}, tenantID interface{}, id interface{}) *MockProfileRepository_FetchProfile_Call {
	return &MockProfileRepository_FetchProfile_Call{Call: _e.mock.On("FetchProfile", ctx, tenantID, id)}
}

func (_c *MockProfileRepository_FetchProfile_Call) Run(run func(ctx context.Context, tenantID uuid.UUID, id uuid.UUID)) *MockProfileRepository_FetchProfile_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(uuid.UUID), args[2].(uuid.UUID))
	})
	return _c
}

func (_c *MockProfileRepository_FetchProfile_Call) Return(pr *profile.Profile, err error) *MockProfileRepository_FetchProfile_Call {
	_c.Call.Return(pr, err)
	return _c
}

func (_c *MockProfileRepository_FetchProfile_Call) RunAndReturn(run func(context.Context, uuid.UUID, uuid.UUID) (*profile.Profile, error)) *MockProfileRepository_FetchProfile_Call {
	_c.Call.Return(run)
	return _c
}

// FindProfileNames provides a mock function with given fields: ctx, tenantID, query
func (_m *MockProfileRepository) FindProfileNames(ctx context.Context, tenantID uuid.UUID, query string) ([]string, error) {
	ret := _m.Called(ctx, tenantID, query)

	if len(ret) == 0 {
		panic("no return value specified for FindProfileNames")
	}

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) ([]string, error)); ok {
		return rf(ctx, tenantID, query)
	}
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) []string); ok {
		r0 = rf(ctx, tenantID, query)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID, string) error); ok {
		r1 = rf(ctx, tenantID, query)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockProfileRepository_FindProfileNames_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindProfileNames'
type MockProfileRepository_FindProfileNames_Call struct {
	*mock.Call
}

// FindProfileNames is a helper method to define mock.On call
//   - ctx context.Context
//   - tenantID uuid.UUID
//   - query string
func (_e *MockProfileRepository_Expecter) FindProfileNames(ctx interface{}, tenantID interface{}, query interface{}) *MockProfileRepository_FindProfileNames_Call {
	return &MockProfileRepository_FindProfileNames_Call{Call: _e.mock.On("FindProfileNames", ctx, tenantID, query)}
}

func (_c *MockProfileRepository_FindProfileNames_Call) Run(run func(ctx context.Context, tenantID uuid.UUID, query string)) *MockProfileRepository_FindProfileNames_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(uuid.UUID), args[2].(string))
	})
	return _c
}

func (_c *MockProfileRepository_FindProfileNames_Call) Return(names []string, err error) *MockProfileRepository_FindProfileNames_Call {
	_c.Call.Return(names, err)
	return _c
}

func (_c *MockProfileRepository_FindProfileNames_Call) RunAndReturn(run func(context.Context, uuid.UUID, string) ([]string, error)) *MockProfileRepository_FindProfileNames_Call {
	_c.Call.Return(run)
	return _c
}

// FindProfilesByName provides a mock function with given fields: ctx, tenantID, name
func (_m *MockProfileRepository) FindProfilesByName(ctx context.Context, tenantID uuid.UUID, name string) ([]*profile.Profile, error) {
	ret := _m.Called(ctx, tenantID, name)

	if len(ret) == 0 {
		panic("no return value specified for FindProfilesByName")
	}

	var r0 []*profile.Profile
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) ([]*profile.Profile, error)); ok {
		return rf(ctx, tenantID, name)
	}
	if rf, ok := ret.Get(0).(func(context.Context, uuid.UUID, string) []*profile.Profile); ok {
		r0 = rf(ctx, tenantID, name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*profile.Profile)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, uuid.UUID, string) error); ok {
		r1 = rf(ctx, tenantID, name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockProfileRepository_FindProfilesByName_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'FindProfilesByName'
type MockProfileRepository_FindProfilesByName_Call struct {
	*mock.Call
}

// FindProfilesByName is a helper method to define mock.On call
//   - ctx context.Context
//   - tenantID uuid.UUID
//   - name string
func (_e *MockProfileRepository_Expecter) FindProfilesByName(ctx interface{}, tenantID interface{}, name interface{}) *MockProfileRepository_FindProfilesByName_Call {
	return &MockProfileRepository_FindProfilesByName_Call{Call: _e.mock.On("FindProfilesByName", ctx, tenantID, name)}
}

func (_c *MockProfileRepository_FindProfilesByName_Call) Run(run func(ctx context.Context, tenantID uuid.UUID, name string)) *MockProfileRepository_FindProfilesByName_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(uuid.UUID), args[2].(string))
	})
	return _c
}

func (_c *MockProfileRepository_FindProfilesByName_Call) Return(prs []*profile.Profile, err error) *MockProfileRepository_FindProfilesByName_Call {
	_c.Call.Return(prs, err)
	return _c
}

func (_c *MockProfileRepository_FindProfilesByName_Call) RunAndReturn(run func(context.Context, uuid.UUID, string) ([]*profile.Profile, error)) *MockProfileRepository_FindProfilesByName_Call {
	_c.Call.Return(run)
	return _c
}

// StoreProfile provides a mock function with given fields: ctx, pr
func (_m *MockProfileRepository) StoreProfile(ctx context.Context, pr *profile.Profile) error {
	ret := _m.Called(ctx, pr)

	if len(ret) == 0 {
		panic("no return value specified for StoreProfile")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *profile.Profile) error); ok {
		r0 = rf(ctx, pr)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockProfileRepository_StoreProfile_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'StoreProfile'
type MockProfileRepository_StoreProfile_Call struct {
	*mock.Call
}

// StoreProfile is a helper method to define mock.On call
//   - ctx context.Context
//   - pr *profile.Profile
func (_e *MockProfileRepository_Expecter) StoreProfile(ctx interface{}, pr interface{}) *MockProfileRepository_StoreProfile_Call {
	return &MockProfileRepository_StoreProfile_Call{Call: _e.mock.On("StoreProfile", ctx, pr)}
}

func (_c *MockProfileRepository_StoreProfile_Call) Run(run func(ctx context.Context, pr *profile.Profile)) *MockProfileRepository_StoreProfile_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*profile.Profile))
	})
	return _c
}

func (_c *MockProfileRepository_StoreProfile_Call) Return(err error) *MockProfileRepository_StoreProfile_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *MockProfileRepository_StoreProfile_Call) RunAndReturn(run func(context.Context, *profile.Profile) error) *MockProfileRepository_StoreProfile_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockProfileRepository creates a new instance of MockProfileRepository. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockProfileRepository(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockProfileRepository {
	mock := &MockProfileRepository{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
