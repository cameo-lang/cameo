package assert

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
)

// JSONDiff represents a difference found during JSON comparison
type JSONDiff struct {
	Path     string
	Expected interface{}
	Actual   interface{}
}

// CompareJSON compares two values via JSON serialization and reports differences by path
func CompareJSON(t *testing.T, actual, expected interface{}) {
	t.Helper()

	actualJSON := mustJSON(actual)
	expectedJSON := mustJSON(expected)

	var actualObj, expectedObj interface{}
	mustDecode(actualJSON, &actualObj)
	mustDecode(expectedJSON, &expectedObj)

	diffs := compareJSONRecursive("", actualObj, expectedObj)
	if len(diffs) > 0 {
		first := diffs[0]
		msg := fmt.Sprintf("JSON mismatch at path '%s':\n  Expected: %v\n  Actual:   %v",
			first.Path, Inspect(first.Expected), Inspect(first.Actual))

		if len(diffs) > 1 {
			msg += fmt.Sprintf("\n\n(Found %d mismatches total)", len(diffs))
		}

		t.Error(msg)
	}
}

// compareJSONRecursive recursively compares two JSON objects and returns all differences
func compareJSONRecursive(path string, actual, expected interface{}) []JSONDiff {
	var diffs []JSONDiff

	// Handle nil cases
	if expected == nil && actual == nil {
		return nil
	}
	if expected == nil || actual == nil {
		return []JSONDiff{{Path: path, Expected: expected, Actual: actual}}
	}

	actualVal := reflect.ValueOf(actual)
	expectedVal := reflect.ValueOf(expected)

	// Check types match
	if actualVal.Kind() != expectedVal.Kind() {
		return []JSONDiff{{Path: path, Expected: expected, Actual: actual}}
	}

	switch expectedVal.Kind() {
	case reflect.Map:
		actualMap := actual.(map[string]interface{})
		expectedMap := expected.(map[string]interface{})

		// Check all keys in expected
		for key, expectedValue := range expectedMap {
			actualValue, exists := actualMap[key]
			keyPath := path
			if keyPath == "" {
				keyPath = key
			} else {
				keyPath = keyPath + "." + key
			}

			if !exists {
				diffs = append(diffs, JSONDiff{Path: keyPath, Expected: expectedValue, Actual: nil})
			} else {
				diffs = append(diffs, compareJSONRecursive(keyPath, actualValue, expectedValue)...)
			}
		}

		// Check for extra keys in actual
		for key := range actualMap {
			if _, exists := expectedMap[key]; !exists {
				keyPath := path
				if keyPath == "" {
					keyPath = key
				} else {
					keyPath = keyPath + "." + key
				}
				diffs = append(diffs, JSONDiff{Path: keyPath, Expected: nil, Actual: actualMap[key]})
			}
		}

	case reflect.Slice:
		actualSlice := actual.([]interface{})
		expectedSlice := expected.([]interface{})

		if len(actualSlice) != len(expectedSlice) {
			diffs = append(diffs, JSONDiff{
				Path:     path + ".length",
				Expected: len(expectedSlice),
				Actual:   len(actualSlice),
			})
			// Continue comparing elements up to the shorter length
		}

		minLen := len(expectedSlice)
		if len(actualSlice) < minLen {
			minLen = len(actualSlice)
		}

		for i := 0; i < minLen; i++ {
			indexPath := fmt.Sprintf("%s[%d]", path, i)
			diffs = append(diffs, compareJSONRecursive(indexPath, actualSlice[i], expectedSlice[i])...)
		}

	default:
		// Compare primitive values
		if !reflect.DeepEqual(actual, expected) {
			diffs = append(diffs, JSONDiff{Path: path, Expected: expected, Actual: actual})
		}
	}

	return diffs
}

// Inspect formats a value for display in error messages
func Inspect(v interface{}) string {
	if v == nil {
		return "<nil>"
	}

	// For strings, show them quoted
	if s, ok := v.(string); ok {
		return fmt.Sprintf("%q", s)
	}

	// For numbers and bools, use default formatting
	switch v.(type) {
	case float64, int, int64, bool:
		return fmt.Sprintf("%v", v)
	}

	// For complex types, marshal to JSON
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// mustJSON returns v as JSON, and panics when it cannot be encoded.
func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// mustDecode decodes b into v, and panics when it cannot be decoded.
func mustDecode(b []byte, v any) {
	if err := json.Unmarshal(b, v); err != nil {
		panic(err)
	}
}
