package structconf

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/urfave/cli/v3"
)

// sliceFlag keeps the normal urfave flag lifecycle, but selects splitting per
// field instead of using the command-wide slice separator setting.
type sliceFlag struct {
	*cli.StringFlag

	field      reflect.StructField
	defaults   reflect.Value
	value      reflect.Value
	repeated   bool
	set        bool
	fromSource bool
	info       FieldInfo
	secret     bool
	count      int
	stringer   cli.FlagStringFunc
}

func (f *sliceFlag) PreParse() error {
	f.value = f.defaults
	f.set = false
	f.fromSource = false
	f.count = 0
	return nil
}

func (f *sliceFlag) Set(_ string, value string) error {
	values := splitSliceValue(value, f.repeated)
	parsed, err := parseSlice(f.field, values)
	if err != nil {
		return err
	}
	if f.repeated && f.set && !f.fromSource {
		f.value = reflect.AppendSlice(f.value, parsed)
	} else {
		f.value = parsed
	}
	if f.fromSource {
		f.count = 0
	}
	f.count++
	f.set = true
	f.fromSource = false
	return nil
}

func (f *sliceFlag) PostParse() error {
	if f.set {
		return nil
	}
	for _, source := range f.Sources.Chain {
		values, found := sliceSourceValues(source, f.repeated)
		if !found {
			continue
		}
		parsed, err := parseSlice(f.field, values)
		if err != nil {
			message := "invalid source for --" + f.Name
			if !f.secret {
				message += ": " + err.Error()
			}
			return &ValidationError{Issues: []ValidationIssue{{
				Message: message,
				Fields:  []FieldInfo{f.info},
			}}}
		}
		f.value, f.set = parsed, true
		f.fromSource = true
		f.count = 1
		break
	}
	return nil
}

func (f *sliceFlag) Get() any               { return f.value.Interface() }
func (f *sliceFlag) IsSet() bool            { return f.set }
func (f *sliceFlag) Count() int             { return f.count }
func (f *sliceFlag) IsMultiValueFlag() bool { return f.repeated }
func (f *sliceFlag) TypeName() string       { return f.field.Type.String() }
func (f *sliceFlag) SchemaType() string     { return "array" }

func (f *sliceFlag) String() string {
	if f.stringer != nil {
		return f.stringer(f)
	}
	return cli.FlagStringer(f)
}

func (f *sliceFlag) SetStringer(stringer cli.FlagStringFunc) {
	f.stringer = stringer
}

func (f *sliceFlag) SchemaItemsType() string {
	t := f.field.Type.Elem()
	if t == reflect.TypeFor[time.Duration]() {
		return "duration"
	}
	switch t.Kind() { //nolint:exhaustive // Only supported scalar types reach this flag.
	case reflect.Bool:
		return "boolean"
	case reflect.String:
		return "string"
	case reflect.Float32, reflect.Float64:
		return "number"
	default:
		return "integer"
	}
}

func splitSliceValue(value string, literal bool) []string {
	if literal {
		return []string{value}
	}
	if value == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func sliceSourceValues(source cli.ValueSource, literal bool) ([]string, bool) {
	if maps, ok := source.(*mapsValueSource); ok {
		for _, m := range maps.maps {
			value, found := m.Lookup(maps.key)
			if !found {
				continue
			}
			v := reflect.ValueOf(value)
			if v.IsValid() && (v.Kind() == reflect.Slice || v.Kind() == reflect.Array) {
				values := make([]string, v.Len())
				for i := range v.Len() {
					values[i] = fmt.Sprint(v.Index(i).Interface())
				}
				return values, true
			}
			return splitSliceValue(fmt.Sprint(value), literal), true
		}
		return nil, false
	}
	value, found := source.Lookup()
	return splitSliceValue(value, literal), found
}

func parseSlice(field reflect.StructField, values []string) (reflect.Value, error) {
	result := reflect.Zero(field.Type)
	if len(values) > 0 {
		result = reflect.MakeSlice(field.Type, len(values), len(values))
	}
	element := field
	element.Type = field.Type.Elem()
	for i, value := range values {
		if err := setFieldValueFromString(element, result.Index(i), value); err != nil {
			return reflect.Value{}, fmt.Errorf("element %d: %w", i, err)
		}
	}
	return result, nil
}
