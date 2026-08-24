package dd

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

const (
	additionalPropertiesField = "AdditionalProperties"
	unparsedObjectField       = "UnparsedObject"
)

// UnknownFields reports the JSON keys of a decoded Datadog model that the
// generated client did not recognize.
//
// The generated models never fail on unexpected input: an object keeps
// unrecognized keys in AdditionalProperties, and a oneOf model that matches no
// variant keeps the whole object in UnparsedObject. Both survive a round trip
// back to Datadog, so a typo in a nested field is otherwise invisible until the
// test silently behaves differently than the file says it should.
//
// Paths are dotted and use the JSON names, with slice indexes, for example
// "options.tick_evry" or "config.steps[0].extractedValues[0].parsr".
func UnknownFields(value any) []string {
	var found []string
	walkUnknown(reflect.ValueOf(value), "", &found)
	sort.Strings(found)
	return found
}

func walkUnknown(value reflect.Value, path string, found *[]string) {
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if value.IsNil() {
			return
		}
		walkUnknown(value.Elem(), path, found)
	case reflect.Slice, reflect.Array:
		for i := 0; i < value.Len(); i++ {
			walkUnknown(value.Index(i), fmt.Sprintf("%s[%d]", path, i), found)
		}
	case reflect.Map:
		for _, key := range value.MapKeys() {
			walkUnknown(value.MapIndex(key), joinFieldPath(path, fmt.Sprint(key.Interface())), found)
		}
	case reflect.Struct:
		walkUnknownStruct(value, path, found)
	}
}

func walkUnknownStruct(value reflect.Value, path string, found *[]string) {
	structType := value.Type()
	for i := 0; i < structType.NumField(); i++ {
		field := structType.Field(i)
		if !field.IsExported() {
			continue
		}
		switch field.Name {
		case additionalPropertiesField:
			for _, key := range mapKeys(value.Field(i)) {
				*found = append(*found, joinFieldPath(path, key))
			}
		case unparsedObjectField:
			if !isNilOrEmpty(value.Field(i)) {
				*found = append(*found, unparsedPath(path))
			}
		default:
			walkUnknown(value.Field(i), joinFieldPath(path, jsonFieldName(field)), found)
		}
	}
}

func mapKeys(value reflect.Value) []string {
	if value.Kind() != reflect.Map || value.IsNil() {
		return nil
	}
	keys := make([]string, 0, value.Len())
	for _, key := range value.MapKeys() {
		keys = append(keys, fmt.Sprint(key.Interface()))
	}
	return keys
}

func isNilOrEmpty(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Pointer, reflect.Interface:
		if value.IsNil() {
			return true
		}
		return isNilOrEmpty(value.Elem())
	case reflect.Map, reflect.Slice:
		return value.IsNil() || value.Len() == 0
	default:
		return !value.IsValid()
	}
}

// jsonFieldName returns "" for a field that is not a JSON key of its own. The
// variant pointers of a generated oneOf model are the case that matters: they
// are transparent in JSON, so they must not show up in a reported path.
func jsonFieldName(field reflect.StructField) string {
	name := strings.Split(field.Tag.Get("json"), ",")[0]
	if name == "-" {
		return ""
	}
	return name
}

func joinFieldPath(path, name string) string {
	switch {
	case name == "":
		return path
	case path == "":
		return name
	default:
		return path + "." + name
	}
}

func unparsedPath(path string) string {
	if path == "" {
		return "(root)"
	}
	return path
}
