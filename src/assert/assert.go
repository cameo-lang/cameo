// Package assert holds the assertions amber's tests use. It is vendored
// from github.com/radiospiel/simple-go/src/assert, under the BSD license in
// LICENSE beside it, with that module's must and preconditions helpers
// inlined so it depends on the standard library alone.
package assert

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
)

// testingT is an interface that abstracts the testing.T API.
// This allows assert functions to work seamlessly with both *testing.T
// (the standard Go test runner) and mock implementations for testing the
// assert package itself. Since *testing.T implements these methods, callers
// can pass *testing.T directly to functions accepting testingT.
//
// All assert functions use Fatal to stop test execution on failure,
// consistent with other assertion packages.
type testingT interface {
	Helper()
	Fatal(args ...interface{})
}

// Equals checks if actual equals expected and fails the test if not.
// Works with any comparable type (strings, ints, bools, etc.)
// details are optional and can provide additional details about the failed assertion, using a format string and values.
func Equals(t testingT, actual any, expected any, details ...interface{}) {
	t.Helper()

	if !isEqual(actual, expected) {
		msg := fmt.Sprintf("assert.Equals(...) failed:\n  Expected: %v\n  Actual:   %v", expected, actual)
		msg = messageWithDetails(msg, details...)
		t.Fatal(msg)
	}
}

// NotEquals checks if actual does not equal expected and fails the test if they are equal.
// details are optional and can provide additional details about the failed assertion, using a format string and values.
func NotEquals[T comparable](t testingT, actual, expected T, details ...interface{}) {
	t.Helper()
	if isEqual(actual, expected) {
		msg := fmt.Sprintf("assert.NotEquals(...) failed:\n  Expected not to equal: %v\n  Actual:                %v", expected, actual)
		msg = messageWithDetails(msg, details...)
		t.Fatal(msg)
	}
}

func isEqual(actual any, expected any) bool {
	if compareAsBytes(actual) && compareAsBytes(expected) {
		return bytes.Equal(bytesFrom(actual), bytesFrom(expected))
	}

	return reflect.DeepEqual(actual, expected)
}

// messageWithDetails prepends a custom message to the default message if details is provided.
func messageWithDetails(msg string, details ...interface{}) string {
	if len(details) > 0 {
		format, ok := details[0].(string)
		if !ok {
			panic(fmt.Sprintf("first argument to custom message must be a format string, got %T", details[0]))
		}

		customMsg := fmt.Sprintf(format, details[1:]...)
		return customMsg + "\n" + msg
	}
	return msg
}

func compareAsBytes(content any) bool {
	switch content.(type) {
	case string:
		return true
	case []byte:
		return true
	default:
		return false
	}
}

func bytesFrom(content any) []byte {
	switch v := content.(type) {
	case string:
		return []byte(v)
	case []byte:
		return v
	default:
		panic(fmt.Sprintf("Cannot convert unsupported type %T to []byte, expected string or []byte", content))
	}
}

// True checks if the condition is true and fails if not.
// details are optional and can provide additional details about the failed assertion, using a format string and values.
func True(t testingT, condition bool, details ...interface{}) {
	t.Helper()
	if !condition {
		msg := "assert.True(...) failed"
		msg = messageWithDetails(msg, details...)
		t.Fatal(msg)
	}
}

// False checks if the condition is false and fails if not.
// details are optional and can provide additional details about the failed assertion, using a format string and values.
func False(t testingT, condition bool, details ...interface{}) {
	t.Helper()
	if condition {
		msg := "assert.False(...) failed"
		msg = messageWithDetails(msg, details...)
		t.Fatal(msg)
	}
}

// Panics calls fn and fails the test if it does not panic.
// details are optional and can provide additional details about the failed assertion, using a format string and values.
func Panics(t testingT, fn func(), details ...interface{}) {
	t.Helper()
	defer func() {
		if recover() == nil {
			msg := "assert.Panics(...) failed: function did not panic"
			msg = messageWithDetails(msg, details...)
			t.Fatal(msg)
		}
	}()
	fn()
}

// Nil checks if the value is nil and fails if not.
// details are optional and can provide additional details about the failed assertion, using a format string and values.
func Nil(t testingT, value interface{}, details ...interface{}) {
	t.Helper()
	if !isNil(value) {
		msg := fmt.Sprintf("assert.Nil(...) failed:\n  Expected: nil\n  Actual:   %v", value)
		msg = messageWithDetails(msg, details...)
		t.Fatal(msg)
	}
}

// NotNil checks if the value is not nil and fails if it is nil.
// details are optional and can provide additional details about the failed assertion, using a format string and values.
func NotNil(t testingT, value interface{}, details ...interface{}) {
	t.Helper()
	if isNil(value) {
		msg := "assert.NotNil(...) failed:\n  Expected: not nil\n  Actual:   nil"
		msg = messageWithDetails(msg, details...)
		t.Fatal(msg)
	}
}

// Helper function to check if a value is nil (handles typed nil pointers)
func isNil(v interface{}) bool {
	if v == nil {
		return true
	}
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func, reflect.Interface:
		return val.IsNil()
	default:
		return false
	}
}

// Error checks if err is not nil and contains the expected error string.
// expectedError is a string that must be present in the error's String() representation.
// details are optional and can provide additional details about the failed assertion, using a format string and values.
func Error(t testingT, err error, expectedError string, details ...interface{}) {
	t.Helper()

	if err != nil && strings.Contains(err.Error(), expectedError) {
		return
	}

	msg := ""
	if err == nil {
		msg = "assert.Error(...) failed:\n  Expected an error but got nil"
	} else {
		msg = fmt.Sprintf("assert.Error(...) failed:\n  Expected error to contain: %q\n  Got:                       %v", expectedError, err)
	}

	msg = messageWithDetails(msg, details...)
	t.Fatal(msg)
}

// NoError checks if err is nil and fails if it's not.
// details are optional and can provide additional details about the failed assertion, using a format string and values.
func NoError(t testingT, err error, details ...interface{}) {
	t.Helper()
	if err != nil {
		msg := fmt.Sprintf("assert.NoError(...) failed:\n  Expected no error\n  Got:      %v", err)
		msg = messageWithDetails(msg, details...)
		t.Fatal(msg)
	}
}

// Contains checks if a container contains the expected element.
// For strings: checks if the string contains the substring.
// For slices/arrays: checks if the slice contains the element.
// details are optional and can provide additional details about the failed assertion, using a format string and values.
func Contains(t testingT, container, element interface{}, details ...interface{}) {
	t.Helper()

	containerVal := reflect.ValueOf(container)

	switch containerVal.Kind() {
	case reflect.String:
		// String substring check
		str := containerVal.String()
		substr, ok := element.(string)
		if !ok {
			msg := fmt.Sprintf("assert.Contains(...) failed:\n  When container is a string, element must also be a string, got %T", element)
			msg = messageWithDetails(msg, details...)
			t.Fatal(msg)
			return
		}
		if !strings.Contains(str, substr) {
			msg := fmt.Sprintf("assert.Contains(...) failed:\n  String:         %q\n  Should contain: %q", str, substr)
			msg = messageWithDetails(msg, details...)
			t.Fatal(msg)
		}

	case reflect.Slice, reflect.Array:
		// Slice/array element check
		found := false
		for i := 0; i < containerVal.Len(); i++ {
			if reflect.DeepEqual(containerVal.Index(i).Interface(), element) {
				found = true
				break
			}
		}
		if !found {
			msg := fmt.Sprintf("assert.Contains(...) failed:\n  Slice:    %v\n  Expected: %v", container, element)
			msg = messageWithDetails(msg, details...)
			t.Fatal(msg)
		}

	default:
		msg := fmt.Sprintf("assert.Contains(...) failed:\n  Unsupported container type: %T (expected string, slice, or array)", container)
		msg = messageWithDetails(msg, details...)
		t.Fatal(msg)
	}
}

// NotContains checks if a container does not contain the expected element.
// For strings: checks if the string does not contain the substring.
// For slices/arrays: checks if the slice does not contain the element.
// details are optional and can provide additional details about the failed assertion, using a format string and values.
func NotContains(t testingT, container, element interface{}, details ...interface{}) {
	t.Helper()

	containerVal := reflect.ValueOf(container)

	switch containerVal.Kind() {
	case reflect.String:
		// String substring check
		str := containerVal.String()
		substr, ok := element.(string)
		if !ok {
			msg := fmt.Sprintf("assert.NotContains(...) failed:\n  When container is a string, element must also be a string, got %T", element)
			msg = messageWithDetails(msg, details...)
			t.Fatal(msg)
			return
		}
		if strings.Contains(str, substr) {
			msg := fmt.Sprintf("assert.NotContains(...) failed:\n  String:             %q\n  Should not contain: %q", str, substr)
			msg = messageWithDetails(msg, details...)
			t.Fatal(msg)
		}

	case reflect.Slice, reflect.Array:
		// Slice/array element check
		for i := 0; i < containerVal.Len(); i++ {
			if reflect.DeepEqual(containerVal.Index(i).Interface(), element) {
				msg := fmt.Sprintf("assert.NotContains(...) failed:\n  Slice:            %v\n  Should not contain: %v", container, element)
				msg = messageWithDetails(msg, details...)
				t.Fatal(msg)
				return
			}
		}

	default:
		msg := fmt.Sprintf("assert.NotContains(...) failed:\n  Unsupported container type: %T (expected string, slice, or array)", container)
		msg = messageWithDetails(msg, details...)
		t.Fatal(msg)
	}
}
