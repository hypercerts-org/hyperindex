package lexicon

import (
	"reflect"
	"testing"
)

func TestZeroValueForType(t *testing.T) {
	tests := []struct {
		name     string
		propType string
		format   string
		want     interface{}
	}{
		{name: "string no format", propType: "string", format: "", want: ""},
		{name: "string datetime format", propType: "string", format: "datetime", want: ""},
		{name: "integer", propType: "integer", format: "", want: 0},
		{name: "boolean", propType: "boolean", format: "", want: false},
		{name: "array", propType: "array", format: "", want: []interface{}{}},
		{name: "ref", propType: "ref", format: "", want: nil},
		{name: "union", propType: "union", format: "", want: nil},
		{name: "blob", propType: "blob", format: "", want: nil},
		{name: "bytes", propType: "bytes", format: "", want: nil},
		{name: "cid-link", propType: "cid-link", format: "", want: nil},
		{name: "unknown", propType: "unknown", format: "", want: nil},
		{name: "object", propType: "object", format: "", want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ZeroValueForType(tt.propType, tt.format)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ZeroValueForType(%q, %q) = %v (%T), want %v (%T)",
					tt.propType, tt.format, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestRecordDefRequiredProperties(t *testing.T) {
	rec := &RecordDef{
		Type: "record",
		Properties: []PropertyEntry{
			{Name: "title", Property: Property{Type: TypeString, Required: true}},
			{Name: "description", Property: Property{Type: TypeString, Required: false}},
			{Name: "count", Property: Property{Type: TypeInteger, Required: true}},
			{Name: "tags", Property: Property{Type: TypeArray, Required: false}},
		},
	}

	required := rec.RequiredProperties()

	if len(required) != 2 {
		t.Fatalf("expected 2 required properties, got %d", len(required))
	}

	names := make([]string, len(required))
	for i, e := range required {
		names[i] = e.Name
	}

	if names[0] != "title" {
		t.Errorf("expected first required property to be 'title', got %q", names[0])
	}
	if names[1] != "count" {
		t.Errorf("expected second required property to be 'count', got %q", names[1])
	}

	// Verify non-required properties are excluded
	for _, e := range required {
		if !e.Property.Required {
			t.Errorf("property %q is not required but was returned by RequiredProperties()", e.Name)
		}
	}
}

func TestRecordDefRequiredPropertiesEmpty(t *testing.T) {
	rec := &RecordDef{
		Type: "record",
		Properties: []PropertyEntry{
			{Name: "optional", Property: Property{Type: TypeString, Required: false}},
		},
	}

	required := rec.RequiredProperties()
	if len(required) != 0 {
		t.Errorf("expected 0 required properties, got %d", len(required))
	}
}
