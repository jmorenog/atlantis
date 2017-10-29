// Automatically generated by pegomock. DO NOT EDIT!
// Source: github.com/hootsuite/atlantis/server/events (interfaces: CommandRunner)

package mocks

import (
	events "github.com/hootsuite/atlantis/server/events"
	pegomock "github.com/petergtz/pegomock"
	"reflect"
)

type MockCommandRunner struct {
	fail func(message string, callerSkip ...int)
}

func NewMockCommandRunner() *MockCommandRunner {
	return &MockCommandRunner{fail: pegomock.GlobalFailHandler}
}

func (mock *MockCommandRunner) ExecuteCommand(ctx *events.CommandContext) {
	params := []pegomock.Param{ctx}
	pegomock.GetGenericMockFrom(mock).Invoke("ExecuteCommand", params, []reflect.Type{})
}

func (mock *MockCommandRunner) VerifyWasCalledOnce() *VerifierCommandRunner {
	return &VerifierCommandRunner{mock, pegomock.Times(1), nil}
}

func (mock *MockCommandRunner) VerifyWasCalled(invocationCountMatcher pegomock.Matcher) *VerifierCommandRunner {
	return &VerifierCommandRunner{mock, invocationCountMatcher, nil}
}

func (mock *MockCommandRunner) VerifyWasCalledInOrder(invocationCountMatcher pegomock.Matcher, inOrderContext *pegomock.InOrderContext) *VerifierCommandRunner {
	return &VerifierCommandRunner{mock, invocationCountMatcher, inOrderContext}
}

type VerifierCommandRunner struct {
	mock                   *MockCommandRunner
	invocationCountMatcher pegomock.Matcher
	inOrderContext         *pegomock.InOrderContext
}

func (verifier *VerifierCommandRunner) ExecuteCommand(ctx *events.CommandContext) *CommandRunner_ExecuteCommand_OngoingVerification {
	params := []pegomock.Param{ctx}
	methodInvocations := pegomock.GetGenericMockFrom(verifier.mock).Verify(verifier.inOrderContext, verifier.invocationCountMatcher, "ExecuteCommand", params)
	return &CommandRunner_ExecuteCommand_OngoingVerification{mock: verifier.mock, methodInvocations: methodInvocations}
}

type CommandRunner_ExecuteCommand_OngoingVerification struct {
	mock              *MockCommandRunner
	methodInvocations []pegomock.MethodInvocation
}

func (c *CommandRunner_ExecuteCommand_OngoingVerification) GetCapturedArguments() *events.CommandContext {
	ctx := c.GetAllCapturedArguments()
	return ctx[len(ctx)-1]
}

func (c *CommandRunner_ExecuteCommand_OngoingVerification) GetAllCapturedArguments() (_param0 []*events.CommandContext) {
	params := pegomock.GetGenericMockFrom(c.mock).GetInvocationParams(c.methodInvocations)
	if len(params) > 0 {
		_param0 = make([]*events.CommandContext, len(params[0]))
		for u, param := range params[0] {
			_param0[u] = param.(*events.CommandContext)
		}
	}
	return
}