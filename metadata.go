package structconf

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// FieldInfo identifies a field without including its value. Name is a flag
// ("--name") or positional argument ("<name>"); Path is its Go field path.
type FieldInfo struct {
	Path string
	Name string
	Help string
}

// Describe resolves field names without loading or modifying config. Config may
// be a struct or a pointer to one, including a typed nil pointer.
func Describe(config any) ([]FieldInfo, error) {
	t := reflect.TypeOf(config)
	if t == nil {
		return nil, errors.New("configuration must be a struct")
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, errors.New("configuration must be a struct")
	}
	return describeStruct(t, nil, make(map[reflect.Type]bool))
}

func describeStruct(t reflect.Type, parents []*configFieldTags, ancestors map[reflect.Type]bool) ([]FieldInfo, error) {
	if ancestors[t] {
		return nil, fmt.Errorf("recursive configuration type %s", t)
	}
	ancestors[t] = true
	defer delete(ancestors, t)
	var fields []FieldInfo
	for field := range t.Fields() {
		if !field.IsExported() {
			continue
		}
		tags := parseTagsWithFieldNameDefault(&field.Tag, field.Name)
		nestedType := field.Type
		if nestedType.Kind() == reflect.Pointer {
			nestedType = nestedType.Elem()
		}
		if nestedType.Kind() == reflect.Struct {
			nested, err := describeStruct(nestedType, append(append([]*configFieldTags{}, parents...), tags), ancestors)
			if err != nil {
				return nil, err
			}
			fields = append(fields, nested...)
			continue
		}
		fields = append(fields, describeField(tags, parents))
	}
	return fields, nil
}

func bindingName(tags *configFieldTags, parents []*configFieldTags) string {
	name := tags.flag
	if tags.arg != "" {
		name = tags.toml
	}
	if tags.isGlobal {
		return name
	}
	parts := make([]string, 0, len(parents)+1)
	for _, parent := range parents {
		parts = append(parts, parent.flag)
	}
	return strings.Join(append(parts, name), "-")
}

func describeField(tags *configFieldTags, parents []*configFieldTags) FieldInfo {
	info := FieldInfo{Help: tags.help}
	path := make([]string, 0, len(parents)+1)
	for _, parent := range parents {
		path = append(path, parent.fieldName)
	}
	info.Path = strings.Join(append(path, tags.fieldName), ".")
	if tags.arg != "" {
		info.Name = "<" + bindingName(tags, parents) + ">"
	} else if tags.flag != "-" {
		info.Name = "--" + bindingName(tags, parents)
	}
	return info
}
