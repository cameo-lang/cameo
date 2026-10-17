package assert

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// mockTestingT is a minimal mock of testing.T for testing assert functions
type mockTestingT struct {
	failed   bool
	errorMsg string
}

func (m *mockTestingT) Helper() {}

func (m *mockTestingT) Fatal(args ...interface{}) {
	m.failed = true
	m.errorMsg = fmt.Sprint(args...)
}

func (m *mockTestingT) expectMockSuccessful(t *testing.T) {
	t.Helper()
	if m.failed {
		t.Errorf("Expected mock to succeed, but it failed with: %s", m.errorMsg)
	}
}

func (m *mockTestingT) expectMockFailure(t *testing.T, expectedSubstring string) {
	t.Helper()
	if !m.failed {
		t.Error("Expected mock to fail, but it succeeded")
	}
	if expectedSubstring != "" && !strings.Contains(m.errorMsg, expectedSubstring) {
		t.Errorf("Expected error message to contain %q, got: %s", expectedSubstring, m.errorMsg)
	}
}

func (m *mockTestingT) resetMock() {
	m.failed = false
	m.errorMsg = ""
}

func TestAssertEquals(t *testing.T) {
	m := &mockTestingT{}

	// These should pass
	Equals(m, 42, 42)
	m.expectMockSuccessful(t)
	m.resetMock()

	Equals(m, "hello", "hello")
	m.expectMockSuccessful(t)
	m.resetMock()

	Equals(m, true, true)
	m.expectMockSuccessful(t)
	m.resetMock()

	// This should fail
	Equals(m, 42, 100)
	m.expectMockFailure(t, "assert.Equals")
}

func TestAssertNotEquals(t *testing.T) {
	m := &mockTestingT{}

	// These should pass
	NotEquals(m, 42, 43)
	m.expectMockSuccessful(t)
	m.resetMock()

	NotEquals(m, "hello", "world")
	m.expectMockSuccessful(t)
	m.resetMock()

	NotEquals(m, true, false)
	m.expectMockSuccessful(t)
}

func TestAssertTrue(t *testing.T) {
	m := &mockTestingT{}

	True(m, true, "Should be true")
	m.expectMockSuccessful(t)
	m.resetMock()

	True(m, 1 == 1, "1 should equal 1")
	m.expectMockSuccessful(t)
	m.resetMock()

	// This should fail
	True(m, false)
	m.expectMockFailure(t, "assert.True")
}

func TestAssertFalse(t *testing.T) {
	m := &mockTestingT{}

	False(m, false, "Should be false")
	m.expectMockSuccessful(t)
	m.resetMock()

	False(m, 1 == 2, "1 should not equal 2")
	m.expectMockSuccessful(t)
	m.resetMock()

	// This should fail
	False(m, true)
	m.expectMockFailure(t, "assert.False")
}

func TestAssertNil(t *testing.T) {
	m := &mockTestingT{}
	var nilPtr *int

	Nil(m, nil, "nil should be nil")
	m.expectMockSuccessful(t)
	m.resetMock()

	Nil(m, nilPtr, "nil pointer should be nil")
	m.expectMockSuccessful(t)
	m.resetMock()

	// This should fail
	Nil(m, "not nil")
	m.expectMockFailure(t, "assert.Nil")
}

func TestAssertNotNil(t *testing.T) {
	m := &mockTestingT{}
	value := 42

	NotNil(m, &value, "pointer should not be nil")
	m.expectMockSuccessful(t)
	m.resetMock()

	NotNil(m, "string", "string should not be nil")
	m.expectMockSuccessful(t)
	m.resetMock()

	// This should fail
	NotNil(m, nil)
	m.expectMockFailure(t, "assert.NotNil")
}

func TestAssertError(t *testing.T) {
	m := &mockTestingT{}

	// Should pass - error contains expected string
	err := errors.New("test error")
	Error(m, err, "test error")
	m.expectMockSuccessful(t)
	m.resetMock()

	// Should pass - error contains substring
	Error(m, err, "test")
	m.expectMockSuccessful(t)
	m.resetMock()

	// Should fail - error is nil
	Error(m, nil, "test error")
	m.expectMockFailure(t, "Expected an error but got nil")
	m.resetMock()

	// Should fail - error doesn't contain expected string
	Error(m, err, "different error")
	m.expectMockFailure(t, "Expected error to contain")
}

func TestAssertNoError(t *testing.T) {
	m := &mockTestingT{}

	NoError(m, nil)
	m.expectMockSuccessful(t)
	m.resetMock()

	// This should fail
	NoError(m, errors.New("some error"))
	m.expectMockFailure(t, "assert.NoError")
}

func TestAssertContainsString(t *testing.T) {
	m := &mockTestingT{}

	Contains(m, "hello world", "world")
	m.expectMockSuccessful(t)
	m.resetMock()

	Contains(m, "package main", "main")
	m.expectMockSuccessful(t)
	m.resetMock()

	// Test failure case
	Contains(m, "hello world", "foo")
	m.expectMockFailure(t, "assert.Contains")
}

func TestAssertEqualsWithCustomMessage(t *testing.T) {
	m := &mockTestingT{}

	// This should fail and include the custom message
	Equals(m, 42, 100, "Expected value to be %d but got %d", 100, 42)
	m.expectMockFailure(t, "Expected value to be 100 but got 42")
}

func TestAssertContains(t *testing.T) {
	m := &mockTestingT{}

	// Test with string slice
	Contains(m, []string{"a", "b", "c"}, "b")
	m.expectMockSuccessful(t)
	m.resetMock()

	// Test with int slice
	Contains(m, []int{1, 2, 3}, 2)
	m.expectMockSuccessful(t)
	m.resetMock()

	// Test failure case
	Contains(m, []string{"a", "b", "c"}, "d")
	m.expectMockFailure(t, "assert.Contains")
}

func TestAssertNotContains(t *testing.T) {
	m := &mockTestingT{}

	// Test with string slice
	NotContains(m, []string{"a", "b", "c"}, "d")
	m.expectMockSuccessful(t)
	m.resetMock()

	// Test failure case
	NotContains(m, []string{"a", "b", "c"}, "b")
	m.expectMockFailure(t, "assert.NotContains")
}
