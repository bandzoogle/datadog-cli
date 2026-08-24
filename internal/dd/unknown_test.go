package dd

import (
	"reflect"
	"testing"
)

type nestedModel struct {
	Value                string `json:"value"`
	UnparsedObject       any
	AdditionalProperties map[string]any
}

type parentModel struct {
	Name                 string        `json:"name"`
	Child                *nestedModel  `json:"child,omitempty"`
	Children             []nestedModel `json:"children,omitempty"`
	AdditionalProperties map[string]any
}

func TestUnknownFieldsReturnsNothingForAKnownModel(t *testing.T) {
	model := parentModel{Name: "ok", Child: &nestedModel{Value: "v"}}
	if got := UnknownFields(model); len(got) != 0 {
		t.Fatalf("expected no unknown fields, got %#v", got)
	}
}

func TestUnknownFieldsReportsNestedPaths(t *testing.T) {
	model := parentModel{
		AdditionalProperties: map[string]any{"nmae": "typo"},
		Child:                &nestedModel{AdditionalProperties: map[string]any{"valu": 1}},
		Children: []nestedModel{
			{Value: "fine"},
			{AdditionalProperties: map[string]any{"vlaue": 2}},
		},
	}

	got := UnknownFields(model)
	want := []string{"child.valu", "children[1].vlaue", "nmae"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestUnknownFieldsReportsUnparsedOneOfObjects(t *testing.T) {
	model := parentModel{
		Children: []nestedModel{
			{Value: "fine"},
			{UnparsedObject: map[string]any{"nope": true}},
		},
	}

	got := UnknownFields(model)
	want := []string{"children[1]"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestUnknownFieldsReportsAnUnparsedRoot(t *testing.T) {
	got := UnknownFields(nestedModel{UnparsedObject: map[string]any{"nope": true}})
	want := []string{"(root)"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

// A generated oneOf model holds its variants in untagged pointer fields, which
// do not exist in the JSON and so must not appear in a reported path.
func TestUnknownFieldsSkipsTransparentOneOfWrappers(t *testing.T) {
	type oneOf struct {
		Variant        *nestedModel
		UnparsedObject any
	}
	type wrapper struct {
		Choice oneOf `json:"choice"`
	}

	model := wrapper{Choice: oneOf{Variant: &nestedModel{AdditionalProperties: map[string]any{"valu": 1}}}}

	got := UnknownFields(model)
	want := []string{"choice.valu"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestUnknownFieldsIgnoresEmptyContainers(t *testing.T) {
	model := parentModel{
		AdditionalProperties: map[string]any{},
		Child:                &nestedModel{UnparsedObject: map[string]any{}},
	}
	if got := UnknownFields(model); len(got) != 0 {
		t.Fatalf("expected no unknown fields, got %#v", got)
	}
}
