package mcp

import (
	"reflect"
	"testing"
)

// TestNewToolResultStructuredOnly_WrapsSlice proves that a slice/array passed
// to NewToolResultStructuredOnly is wrapped in {"items": ...}, because the MCP
// protocol requires structuredContent to be a JSON object, not an array.
// This is the helper the typed StructuredHandler path emits, so an unwrapped
// slice would trigger "expected record, received array" on clients.
func TestNewToolResultStructuredOnly_WrapsSlice(t *testing.T) {
	data := []string{"a", "b", "c"}
	res := NewToolResultStructuredOnly(data)

	wrapped, ok := res.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("expected structuredContent to be wrapped object, got %T", res.StructuredContent)
	}
	got, ok := wrapped["items"]
	if !ok {
		t.Fatalf("expected wrapped object to contain \"items\" key, got %v", wrapped)
	}
	if !reflect.DeepEqual(got, data) {
		t.Fatalf("expected items to equal %v, got %v", data, got)
	}
}

// TestNewToolResultStructuredOnly_ObjectUntouched proves a non-slice value is
// passed through unchanged (no spurious wrapping).
func TestNewToolResultStructuredOnly_ObjectUntouched(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}
	data := payload{Name: "x"}
	res := NewToolResultStructuredOnly(data)

	if _, wrapped := res.StructuredContent.(map[string]any); wrapped {
		// A struct must not be turned into an {"items": ...} wrapper.
		if v, ok := res.StructuredContent.(map[string]any); ok {
			if _, hasItems := v["items"]; hasItems {
				t.Fatalf("object value was wrongly wrapped in items: %v", v)
			}
		}
	}
	if !reflect.DeepEqual(res.StructuredContent, data) {
		t.Fatalf("expected object passed through unchanged, got %v", res.StructuredContent)
	}
}
