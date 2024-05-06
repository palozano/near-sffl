// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/NethermindEth/near-sffl/operator (interfaces: AggregatorRpcClienter)
//
// Generated by this command:
//
//	mockgen -destination=./mocks/rpc_client.go -package=mocks github.com/NethermindEth/near-sffl/operator AggregatorRpcClienter
//
// Package mocks is a generated GoMock package.
package mocks

import (
	reflect "reflect"

	messages "github.com/NethermindEth/near-sffl/core/types/messages"
	prometheus "github.com/prometheus/client_golang/prometheus"
	gomock "go.uber.org/mock/gomock"
)

// MockAggregatorRpcClienter is a mock of AggregatorRpcClienter interface.
type MockAggregatorRpcClienter struct {
	ctrl     *gomock.Controller
	recorder *MockAggregatorRpcClienterMockRecorder
}

// MockAggregatorRpcClienterMockRecorder is the mock recorder for MockAggregatorRpcClienter.
type MockAggregatorRpcClienterMockRecorder struct {
	mock *MockAggregatorRpcClienter
}

// NewMockAggregatorRpcClienter creates a new mock instance.
func NewMockAggregatorRpcClienter(ctrl *gomock.Controller) *MockAggregatorRpcClienter {
	mock := &MockAggregatorRpcClienter{ctrl: ctrl}
	mock.recorder = &MockAggregatorRpcClienterMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAggregatorRpcClienter) EXPECT() *MockAggregatorRpcClienterMockRecorder {
	return m.recorder
}

// EnableMetrics mocks base method.
func (m *MockAggregatorRpcClienter) EnableMetrics(arg0 *prometheus.Registry) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnableMetrics", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// EnableMetrics indicates an expected call of EnableMetrics.
func (mr *MockAggregatorRpcClienterMockRecorder) EnableMetrics(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnableMetrics", reflect.TypeOf((*MockAggregatorRpcClienter)(nil).EnableMetrics), arg0)
}

// GetAggregatedCheckpointMessages mocks base method.
func (m *MockAggregatorRpcClienter) GetAggregatedCheckpointMessages(arg0, arg1 uint64) (*messages.CheckpointMessages, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetAggregatedCheckpointMessages", arg0, arg1)
	ret0, _ := ret[0].(*messages.CheckpointMessages)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetAggregatedCheckpointMessages indicates an expected call of GetAggregatedCheckpointMessages.
func (mr *MockAggregatorRpcClienterMockRecorder) GetAggregatedCheckpointMessages(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetAggregatedCheckpointMessages", reflect.TypeOf((*MockAggregatorRpcClienter)(nil).GetAggregatedCheckpointMessages), arg0, arg1)
}

// SendSignedCheckpointTaskResponseToAggregator mocks base method.
func (m *MockAggregatorRpcClienter) SendSignedCheckpointTaskResponseToAggregator(arg0 *messages.SignedCheckpointTaskResponse) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SendSignedCheckpointTaskResponseToAggregator", arg0)
}

// SendSignedCheckpointTaskResponseToAggregator indicates an expected call of SendSignedCheckpointTaskResponseToAggregator.
func (mr *MockAggregatorRpcClienterMockRecorder) SendSignedCheckpointTaskResponseToAggregator(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendSignedCheckpointTaskResponseToAggregator", reflect.TypeOf((*MockAggregatorRpcClienter)(nil).SendSignedCheckpointTaskResponseToAggregator), arg0)
}

// SendSignedOperatorSetUpdateToAggregator mocks base method.
func (m *MockAggregatorRpcClienter) SendSignedOperatorSetUpdateToAggregator(arg0 *messages.SignedOperatorSetUpdateMessage) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SendSignedOperatorSetUpdateToAggregator", arg0)
}

// SendSignedOperatorSetUpdateToAggregator indicates an expected call of SendSignedOperatorSetUpdateToAggregator.
func (mr *MockAggregatorRpcClienterMockRecorder) SendSignedOperatorSetUpdateToAggregator(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendSignedOperatorSetUpdateToAggregator", reflect.TypeOf((*MockAggregatorRpcClienter)(nil).SendSignedOperatorSetUpdateToAggregator), arg0)
}

// SendSignedStateRootUpdateToAggregator mocks base method.
func (m *MockAggregatorRpcClienter) SendSignedStateRootUpdateToAggregator(arg0 *messages.SignedStateRootUpdateMessage) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SendSignedStateRootUpdateToAggregator", arg0)
}

// SendSignedStateRootUpdateToAggregator indicates an expected call of SendSignedStateRootUpdateToAggregator.
func (mr *MockAggregatorRpcClienterMockRecorder) SendSignedStateRootUpdateToAggregator(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SendSignedStateRootUpdateToAggregator", reflect.TypeOf((*MockAggregatorRpcClienter)(nil).SendSignedStateRootUpdateToAggregator), arg0)
}
