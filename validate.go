package structconf

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/urfave/cli/v3"
)

// ValidationIssue describes a failed constraint, possibly involving several
// fields. Rule and Param are optional; input values are deliberately excluded.
type ValidationIssue struct {
	Message string
	Fields  []FieldInfo
	Rule    string
	Param   string
}

// ValidationError can be returned by both structconf and application validators.
// Applications may construct it directly for inputs not bound by structconf.
type ValidationError struct {
	Issues []ValidationIssue
}

// NewValidationError resolves Go field paths to their CLI names and help text.
// Unknown paths retain their Go name, without inventing a CLI binding. For
// manually defined inputs, construct ValidationError with explicit FieldInfo.
func NewValidationError(message string, config any, paths ...string) *ValidationError {
	metadata := fieldInfoByPath(config)
	fields := make([]FieldInfo, 0, len(paths))
	for _, path := range paths {
		fields = append(fields, resolveFieldInfo(reflect.TypeOf(config), path, metadata))
	}
	return &ValidationError{Issues: []ValidationIssue{{Message: message, Fields: fields}}}
}

func fieldInfoByPath(config any) map[string]FieldInfo {
	fields, _ := Describe(config)
	metadata := make(map[string]FieldInfo, len(fields))
	for _, field := range fields {
		metadata[field.Path] = field
	}
	return metadata
}

func resolveFieldInfo(t reflect.Type, path string, metadata map[string]FieldInfo) FieldInfo {
	info, ok := metadata[resolveFieldPath(t, path)]
	if !ok {
		return FieldInfo{Path: path}
	}
	if strings.Contains(path, "[") {
		info.Path = path
	}
	return info
}

func (e *ValidationError) Error() string {
	messages := make([]string, len(e.Issues))
	for i, issue := range e.Issues {
		messages[i] = issue.Message
	}
	return strings.Join(messages, "\n")
}

// Resolve promoted fields through reflect's field indexes, rather than guessing
// by suffix. A primitive slice's indexed validation failure uses its flag help.
func resolveFieldPath(t reflect.Type, path string) string {
	var names []string
	for part := range strings.SplitSeq(path, ".") {
		if t == nil {
			return path
		}
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return path
		}
		name, suffix, indexed := strings.Cut(part, "[")
		field, ok := t.FieldByName(name)
		if !ok {
			return path
		}
		for _, index := range field.Index {
			if t.Kind() == reflect.Pointer {
				t = t.Elem()
			}
			f := t.Field(index)
			names = append(names, f.Name)
			t = f.Type
		}
		if indexed {
			if t.Kind() != reflect.Slice && t.Kind() != reflect.Array {
				return path
			}
			indexText, closed := strings.CutSuffix(suffix, "]")
			index, err := strconv.Atoi(indexText)
			if !closed || err != nil || strings.Trim(indexText, "0123456789") != "" || index < 0 || (t.Kind() == reflect.Array && index >= t.Len()) {
				return path
			}
			t = t.Elem()
		}
	}
	return strings.Join(names, ".")
}

func validate(_ *cli.Command, configPointer any) error {
	return Validate(configPointer)
}

// Validate applies validate tags and returns a structured ValidationError.
// Custom validators may call it before performing application-specific checks.
func Validate(configPointer any) error {
	configValidator := validator.New(validator.WithRequiredStructEnabled())

	err := configValidator.Struct(configPointer)
	if err != nil {
		if validationErrors, ok := errors.AsType[validator.ValidationErrors](err); ok {
			result := &ValidationError{}
			metadata := fieldInfoByPath(configPointer)
			t := reflect.TypeOf(configPointer)
			if t.Kind() == reflect.Pointer {
				t = t.Elem()
			}

			for _, fieldError := range validationErrors {
				path := strings.TrimPrefix(fieldError.StructNamespace(), t.Name()+".")
				issue := ValidationIssue{
					Fields: []FieldInfo{resolveFieldInfo(t, path, metadata)},
					Rule:   fieldError.ActualTag(),
					Param:  fieldError.Param(),
				}
				issue.Message = validationMessage(issue.Fields[0], fieldError)
				result.Issues = append(result.Issues, issue)
			}

			return result
		}
		return err
	}

	return nil
}

func validationMessage(field FieldInfo, failure validator.FieldError) string {
	name := field.Name
	if name == "" {
		name = field.Path
	}
	param := failure.Param()
	switch failure.ActualTag() {
	case "required":
		return name + " is required"
	case "oneof", "oneofci":
		return name + " must be one of: " + param
	case "min", "max", "len", "gte", "lte", "gt", "lt":
		comparison := map[string]string{
			"min": "at least", "gte": "at least", "max": "at most", "lte": "at most",
			"len": "exactly", "gt": "more than", "lt": "fewer than",
		}[failure.ActualTag()]
		switch failure.Kind() { //nolint:exhaustive // Other kinds retain the generic constraint message.
		case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
			unit := "items"
			if failure.Kind() == reflect.String {
				unit = "characters"
			}
			if param == "1" {
				unit = strings.TrimSuffix(unit, "s")
			}
			return fmt.Sprintf("%s must contain %s %s %s", name, comparison, param, unit)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
			reflect.Float32, reflect.Float64:
			if failure.ActualTag() == "gt" {
				comparison = "greater than"
			} else if failure.ActualTag() == "lt" {
				comparison = "less than"
			}
			return fmt.Sprintf("%s must be %s %s", name, comparison, param)
		}
	case "eq", "ne":
		comparison := "must equal"
		if failure.ActualTag() == "ne" {
			comparison = "must not equal"
		}
		switch failure.Kind() { //nolint:exhaustive // Other kinds use the literal constraint parameter.
		case reflect.String:
			param = strconv.Quote(param)
		case reflect.Slice, reflect.Array, reflect.Map:
			comparison = "must contain exactly"
			if failure.ActualTag() == "ne" {
				comparison = "must not contain exactly"
			}
			if param == "1" {
				param += " item"
			} else {
				param += " items"
			}
		}
		return fmt.Sprintf("%s %s %s", name, comparison, param)
	case "uuid":
		return name + " must be a valid UUID"
	case "url":
		return name + " must be a valid URL"
	case "email":
		return name + " must be a valid email address"
	case "alpha":
		return name + " must contain only letters"
	case "alphanum":
		return name + " must contain only letters and digits"
	case "contains":
		return fmt.Sprintf("%s must contain %q", name, param)
	case "excludes":
		return fmt.Sprintf("%s must not contain %q", name, param)
	case "startswith":
		return fmt.Sprintf("%s must start with %q", name, param)
	case "endswith":
		return fmt.Sprintf("%s must end with %q", name, param)
	}
	return fmt.Sprintf("%s does not satisfy the %s constraint", name, failure.Tag())
}
