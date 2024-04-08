// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/NethermindEth/near-sffl/aggregator/database (interfaces: Databaser)
//
// Generated by this command:
//
//	mockgen -destination=./mocks/database.go -package=mocks github.com/NethermindEth/near-sffl/aggregator/database Databaser
//
// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	messages "github.com/NethermindEth/near-sffl/core/types/messages"
	gomock "go.uber.org/mock/gomock"
)

// MockDatabaser is a mock of Databaser interface.
type MockDatabaser struct {
	ctrl     *gomock.Controller
	recorder *MockDatabaserMockRecorder
}

// MockDatabaserMockRecorder is the mock recorder for MockDatabaser.
type MockDatabaserMockRecorder struct {
	mock *MockDatabaser
}

// NewMockDatabaser creates a new mock instance.
func NewMockDatabaser(ctrl *gomock.Controller) *MockDatabaser {
	mock := &MockDatabaser{ctrl: ctrl}
	mock.recorder = &MockDatabaserMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDatabaser) EXPECT() *MockDatabaserMockRecorder {
	return m.recorder
}

// Close mocks base method.
func (m *MockDatabaser) Close() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Close")
	ret0, _ := ret[0].(error)
	return ret0
}

// Close indicates an expected call of Close.
func (mr *MockDatabaserMockRecorder) Close() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Close", reflect.TypeOf((*MockDatabaser)(nil).Close))
}

// FetchCheckpointMessages mocks base method.
func (m *MockDatabaser) FetchCheckpointMessages(arg0, arg1 uint64) (*messages.CheckpointMessages, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchCheckpointMessages", arg0, arg1)
	ret0, _ := ret[0].(*messages.CheckpointMessages)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchCheckpointMessages indicates an expected call of FetchCheckpointMessages.
func (mr *MockDatabaserMockRecorder) FetchCheckpointMessages(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchCheckpointMessages", reflect.TypeOf((*MockDatabaser)(nil).FetchCheckpointMessages), arg0, arg1)
}

// FetchOperatorSetUpdate mocks base method.
func (m *MockDatabaser) FetchOperatorSetUpdate(arg0 uint64) (*messages.OperatorSetUpdateMessage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchOperatorSetUpdate", arg0)
	ret0, _ := ret[0].(*messages.OperatorSetUpdateMessage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchOperatorSetUpdate indicates an expected call of FetchOperatorSetUpdate.
func (mr *MockDatabaserMockRecorder) FetchOperatorSetUpdate(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchOperatorSetUpdate", reflect.TypeOf((*MockDatabaser)(nil).FetchOperatorSetUpdate), arg0)
}

// FetchOperatorSetUpdateAggregation mocks base method.
func (m *MockDatabaser) FetchOperatorSetUpdateAggregation(arg0 uint64) (*messages.MessageBlsAggregation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchOperatorSetUpdateAggregation", arg0)
	ret0, _ := ret[0].(*messages.MessageBlsAggregation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchOperatorSetUpdateAggregation indicates an expected call of FetchOperatorSetUpdateAggregation.
func (mr *MockDatabaserMockRecorder) FetchOperatorSetUpdateAggregation(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchOperatorSetUpdateAggregation", reflect.TypeOf((*MockDatabaser)(nil).FetchOperatorSetUpdateAggregation), arg0)
}

// FetchStateRootUpdate mocks base method.
func (m *MockDatabaser) FetchStateRootUpdate(arg0 uint32, arg1 uint64) (*messages.StateRootUpdateMessage, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchStateRootUpdate", arg0, arg1)
	ret0, _ := ret[0].(*messages.StateRootUpdateMessage)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchStateRootUpdate indicates an expected call of FetchStateRootUpdate.
func (mr *MockDatabaserMockRecorder) FetchStateRootUpdate(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchStateRootUpdate", reflect.TypeOf((*MockDatabaser)(nil).FetchStateRootUpdate), arg0, arg1)
}

// FetchStateRootUpdateAggregation mocks base method.
func (m *MockDatabaser) FetchStateRootUpdateAggregation(arg0 uint32, arg1 uint64) (*messages.MessageBlsAggregation, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "FetchStateRootUpdateAggregation", arg0, arg1)
	ret0, _ := ret[0].(*messages.MessageBlsAggregation)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// FetchStateRootUpdateAggregation indicates an expected call of FetchStateRootUpdateAggregation.
func (mr *MockDatabaserMockRecorder) FetchStateRootUpdateAggregation(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "FetchStateRootUpdateAggregation", reflect.TypeOf((*MockDatabaser)(nil).FetchStateRootUpdateAggregation), arg0, arg1)
}

// StoreOperatorSetUpdate mocks base method.
func (m *MockDatabaser) StoreOperatorSetUpdate(arg0 messages.OperatorSetUpdateMessage) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StoreOperatorSetUpdate", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// StoreOperatorSetUpdate indicates an expected call of StoreOperatorSetUpdate.
func (mr *MockDatabaserMockRecorder) StoreOperatorSetUpdate(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StoreOperatorSetUpdate", reflect.TypeOf((*MockDatabaser)(nil).StoreOperatorSetUpdate), arg0)
}

// StoreOperatorSetUpdateAggregation mocks base method.
func (m *MockDatabaser) StoreOperatorSetUpdateAggregation(arg0 messages.OperatorSetUpdateMessage, arg1 messages.MessageBlsAggregation) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StoreOperatorSetUpdateAggregation", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// StoreOperatorSetUpdateAggregation indicates an expected call of StoreOperatorSetUpdateAggregation.
func (mr *MockDatabaserMockRecorder) StoreOperatorSetUpdateAggregation(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StoreOperatorSetUpdateAggregation", reflect.TypeOf((*MockDatabaser)(nil).StoreOperatorSetUpdateAggregation), arg0, arg1)
}

// StoreStateRootUpdate mocks base method.
func (m *MockDatabaser) StoreStateRootUpdate(arg0 messages.StateRootUpdateMessage) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StoreStateRootUpdate", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// StoreStateRootUpdate indicates an expected call of StoreStateRootUpdate.
func (mr *MockDatabaserMockRecorder) StoreStateRootUpdate(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StoreStateRootUpdate", reflect.TypeOf((*MockDatabaser)(nil).StoreStateRootUpdate), arg0)
}

// StoreStateRootUpdateAggregation mocks base method.
func (m *MockDatabaser) StoreStateRootUpdateAggregation(arg0 messages.StateRootUpdateMessage, arg1 messages.MessageBlsAggregation) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StoreStateRootUpdateAggregation", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// StoreStateRootUpdateAggregation indicates an expected call of StoreStateRootUpdateAggregation.
func (mr *MockDatabaserMockRecorder) StoreStateRootUpdateAggregation(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StoreStateRootUpdateAggregation", reflect.TypeOf((*MockDatabaser)(nil).StoreStateRootUpdateAggregation), arg0, arg1)
}
