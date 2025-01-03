package structconf

import (
	"fmt"
	"log/slog"
	"reflect"
	"time"
)

func MarshalAsMap(configPointer any) (map[string]any, error) {
	marshalled := make(map[string]any)
	err := marshalStruct(configPointer, marshalled, func(t *configFieldTags) string {
		return t.flag // kebab case
	})
	if err != nil {
		return nil, err
	}

	return marshalled, nil
}

func MarshalAsSlogDict(configPointer any, groupName string) (slog.Attr, error) {
	asMap, err := MarshalAsMap(configPointer)
	if err != nil {
		return slog.Attr{}, err
	}
	attrs := mapToSlogAttrs(asMap)
	return slog.Group(groupName, attrs...), nil
}

func marshalStruct(anyStruct any, into map[string]any, nameFromTags func(t *configFieldTags) string) error {
	structType := reflect.TypeOf(anyStruct)
	structValues := reflect.ValueOf(anyStruct)

	if structType.Kind() == reflect.Ptr {
		structType = structType.Elem()
		structValues = structValues.Elem()
	}

	for i := range structType.NumField() {
		fieldType := structType.Field(i)
		fieldValue := structValues.Field(i)

		isExported := len(fieldType.Name) > 0 && fieldType.Name[0] >= 'A' && fieldType.Name[0] <= 'Z'
		if !isExported { // only marshal exported fields
			continue
		}

		tags := parseTagsWithFieldNameDefault(&fieldType.Tag, fieldType.Name)
		fieldName := nameFromTags(tags)
		if fieldName == "-" { // skip fields tagged with `flag:"-"`
			continue
		}

		if fieldType.Type.Kind() == reflect.Struct {
			// recurse using the pointer to the nested struct, so we can modify it
			nested := make(map[string]any)
			into[fieldName] = nested
			err := marshalStruct(fieldValue.Addr().Interface(), nested, nameFromTags)
			if err != nil {
				return err
			}
			continue
		}

		if fieldType.Type.Kind() == reflect.Ptr && fieldType.Type.Elem().Kind() == reflect.Struct {
			if fieldValue.IsNil() {
				fieldValue.Set(reflect.New(fieldType.Type.Elem()))
			}
			nested := make(map[string]any)
			into[fieldName] = nested
			err := marshalStruct(fieldValue.Interface(), nested, nameFromTags)
			if err != nil {
				return err
			}
			continue
		}

		if fieldType.Type.Kind() != reflect.Bool && fieldValue.IsZero() {
			// don't marshal zero values (except for bools which are false)
			continue
		}

		value, err := getFieldValue(fieldType, fieldValue, tags)
		if err != nil {
			return err
		}
		into[fieldName] = value
	}

	return nil
}

func getFieldValue(field reflect.StructField, fieldValue reflect.Value, tags *configFieldTags) (any, error) {
	switch field.Type.Kind() { //nolint:exhaustive
	case reflect.String:
		if tags.isSecret {
			return redactSecret(fieldValue.String()), nil
		}
		return fieldValue.String(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return fieldValue.Int(), nil
	case reflect.Int64:
		if dur, ok := fieldValue.Interface().(time.Duration); ok { // special handling for time.Duration, which is a int64
			return dur.String(), nil
		}
		return fieldValue.Int(), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fieldValue.Uint(), nil
	case reflect.Float32, reflect.Float64:
		return fieldValue.Float(), nil
	case reflect.Bool:
		return fieldValue.Bool(), nil
	default:
		return nil, fmt.Errorf("unknown field type %s", field.Type.Kind())
	}
}

func mapToSlogAttrs(m map[string]any) []any {
	attrs := make([]any, 0, len(m))
	for key, value := range m {
		if nested, ok := value.(map[string]any); ok {
			nestedAttrs := mapToSlogAttrs(nested)
			attrs = append(attrs, slog.Group(key, nestedAttrs...))
		} else {
			attrs = append(attrs, slog.String(key, fmt.Sprintf("%v", value)))
		}
	}
	return attrs
}

// redactSecret redacts the secret from the given string
// Example:
//
//	secret = "a-secret-key"
//	redacted = redactSecret(secret)
//	redacted = "a-***-ey"
func redactSecret(secret string) string {
	if len(secret) < 5 {
		return "***"
	}

	return secret[0:2] + "***" + secret[len(secret)-2:]
}
