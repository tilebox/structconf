package structconf

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"
	"github.com/urfave/cli/v3"
)

type StructReflector interface {
	Flags() []cli.Flag
	Arguments() []cli.Argument
	Apply(*cli.Command)
}

type ArgumentMetadata interface {
	Name() string
	TypeName() string
	UsageText() string
}

type structReflector struct {
	foundFlags     []cli.Flag           // flags found in the struct
	foundArguments []*configArgument    // arguments found in the struct
	applyFuncs     []func(*cli.Command) // functions to call after flags are parsed, to apply values to the struct

	tomlSources []cli.MapSource
	options     commandOptions
}

func (r *structReflector) Flags() []cli.Flag {
	return r.foundFlags
}

func (r *structReflector) Arguments() []cli.Argument {
	slices.SortFunc(r.foundArguments, func(a, b *configArgument) int {
		return a.index - b.index
	})

	arguments := make([]cli.Argument, 0, len(r.foundArguments))
	for _, argument := range r.foundArguments {
		arguments = append(arguments, argument)
	}

	return arguments
}

func (r *structReflector) Apply(command *cli.Command) {
	for _, applyFunc := range r.applyFuncs {
		applyFunc(command)
	}
}

type configArgument struct {
	index        int
	argumentName string
	usageText    string
	field        reflect.StructField
	fieldValue   reflect.Value
	sources      cli.ValueSourceChain
	defaultValue string
	hasDefault   bool
	nargs        string
	min          int
	max          int
	info         FieldInfo
	secret       bool
}

func (a *configArgument) HasName(name string) bool {
	return name == a.argumentName
}

func (a *configArgument) Parse(args []string) ([]string, error) {
	rest, err := a.parse(args)
	if err != nil {
		message := err.Error()
		if a.secret {
			message = "invalid value for argument " + a.argumentName
		}
		return rest, &ValidationError{Issues: []ValidationIssue{{Message: message, Fields: []FieldInfo{a.info}}}}
	}
	return rest, nil
}

func (a *configArgument) Usage() string {
	switch a.nargs {
	case "?":
		return "[" + a.argumentName + "]"
	case "*":
		return "[" + a.argumentName + " ...]"
	case "+":
		return a.argumentName + " [" + a.argumentName + " ...]"
	case "":
		return a.argumentName
	default:
		return strings.TrimSpace(strings.Repeat(a.argumentName+" ", a.max))
	}
}

func (a *configArgument) Get() any {
	if a.fieldValue.Kind() == reflect.Pointer {
		if a.fieldValue.IsNil() {
			return reflect.Zero(a.field.Type.Elem()).Interface()
		}
		return a.fieldValue.Elem().Interface()
	}
	return a.fieldValue.Interface()
}

func (a *configArgument) Name() string {
	return a.argumentName
}

func (a *configArgument) TypeName() string {
	return typeName(a.field.Type)
}

func (a *configArgument) UsageText() string {
	return a.usageText
}

func (a *configArgument) parse(args []string) ([]string, error) {
	if a.field.Type.Kind() == reflect.Pointer || a.nargs != "" {
		a.fieldValue.SetZero()
	}
	if len(args) > 0 {
		count := len(args)
		if a.max >= 0 {
			count = min(count, a.max)
		}
		if err := a.setValues(args[:count], true); err != nil {
			return args, fmt.Errorf("invalid value %q for argument %s: %w", args[0], a.argumentName, err)
		}
		return args[count:], nil
	}

	for _, source := range a.sources.Chain {
		if a.field.Type.Kind() == reflect.Slice {
			values, found := sliceSourceValues(source, false)
			if !found {
				continue
			}
			return args, a.setValues(values, false)
		}
		value, found := source.Lookup()
		if !found {
			continue
		}
		if value == "" {
			t := a.field.Type
			if t.Kind() == reflect.Pointer {
				t = t.Elem()
			}
			kind := t.Kind()
			if kind == reflect.Bool {
				value = "false"
			} else if kind != reflect.String && kind != reflect.Slice {
				continue
			}
			// String slices accept empty values like their StringFlag counterparts.
		}
		if err := a.setValues([]string{value}, false); err != nil {
			return args, fmt.Errorf("failed to parse source value %s for argument %s: %w", value, a.argumentName, err)
		}

		return args, nil
	}

	if a.hasDefault {
		values := []string{a.defaultValue}
		if a.field.Type.Kind() == reflect.Slice {
			values = splitSliceValue(a.defaultValue, false)
		}
		if err := a.setValues(values, false); err != nil {
			return args, fmt.Errorf("failed to parse default value %s for argument %s: %w", a.defaultValue, a.argumentName, err)
		}
		return args, nil
	}

	return args, a.checkCount(0)
}

func (a *configArgument) checkCount(count int) error {
	if count < a.min || (a.nargs != "" && a.max >= 0 && count > a.max) {
		return fmt.Errorf("argument %s expects %s values, got %d", a.argumentName, a.nargs, count)
	}
	return nil
}

func (a *configArgument) setValues(values []string, fromCLI bool) error {
	if err := a.checkCount(len(values)); err != nil {
		return err
	}
	if a.field.Type.Kind() == reflect.Slice {
		if fromCLI && a.nargs == "" {
			values = splitSliceValue(values[0], false)
		}
		parsed, err := parseSlice(a.field, values)
		if err != nil {
			return err
		}
		a.fieldValue.Set(parsed)
		return nil
	}
	return setFieldValueFromString(a.field, a.fieldValue, values[0])
}

func typeName(valueType reflect.Type) string {
	if valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	return valueType.String()
}

func (r *structReflector) processField(field reflect.StructField, fieldValue reflect.Value, tags *configFieldTags, parents []*configFieldTags) error {
	if tags == nil {
		return nil
	}
	if tags.flag == "-" && tags.arg == "" {
		return nil
	}
	if err := validateSupportedFieldType(field); err != nil {
		return err
	}
	valueType := field.Type
	if valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
	}
	if tags.sliceMode != "" && (valueType.Kind() != reflect.Slice || (tags.sliceMode != "comma-separated" && tags.sliceMode != "repeated-flag")) {
		return fmt.Errorf("invalid slice mode %q for field %s", tags.sliceMode, field.Name)
	}
	if tags.arg != "" && tags.sliceMode == "repeated-flag" {
		return fmt.Errorf("slice mode repeated-flag requires a flag: %s", field.Name)
	}
	if tags.nargs != "" && tags.arg == "" {
		return fmt.Errorf("nargs requires an argument: %s", field.Name)
	}
	if tags.file && (valueType.Kind() != reflect.String || tags.arg != "") {
		return fmt.Errorf("file requires a string flag: %s", field.Name)
	}
	if raw, ok := field.Tag.Lookup("file"); ok {
		if _, err := strconv.ParseBool(raw); err != nil {
			return fmt.Errorf("invalid file tag for %s: %w", field.Name, err)
		}
	}

	valueSources := r.valueSources(tags, parents)

	if tags.arg != "" {
		return r.processArgument(field, fieldValue, tags, parents, valueSources)
	}

	if tags.flag == "-" {
		return nil
	}

	return r.processFlag(field, fieldValue, tags, parents, valueSources)
}

func (r *structReflector) valueSources(tags *configFieldTags, parents []*configFieldTags) []cli.ValueSource {
	if r.options.skipSources {
		return nil
	}
	valueSources := make([]cli.ValueSource, 0)

	if tags.toml != "" && tags.toml != "-" && len(r.tomlSources) > 0 { // load from toml file unless explicitly set to "-"
		tomlKey := tags.toml
		if !tags.isGlobal {
			tomlKeys := lo.Map(parents, func(parent *configFieldTags, _ int) string { return parent.toml })
			tomlKeys = append(tomlKeys, tags.toml)
			tomlKey = strings.Join(tomlKeys, ".")
		}

		valueSources = append(valueSources, NewValueSourceFromMaps(tomlKey, r.tomlSources...))
	}

	if tags.env != "-" && (!r.options.disableImplicitEnvVars || tags.explicitEnv) {
		envKey := tags.env
		if !tags.isGlobal {
			emvKeys := lo.Map(parents, func(parent *configFieldTags, _ int) string { return parent.env })
			emvKeys = append(emvKeys, tags.env)
			envKey = strings.Join(emvKeys, "_")
		}

		valueSources = append(valueSources, cli.EnvVar(envKey))
	}

	return valueSources
}

func (r *structReflector) processArgument(field reflect.StructField, fieldValue reflect.Value, tags *configFieldTags, parents []*configFieldTags, valueSources []cli.ValueSource) error {
	if tags.hasExplicitFlag && tags.flag != "-" {
		return fmt.Errorf("field %s cannot be bound as both argument and flag", field.Name)
	}

	index, err := strconv.Atoi(tags.arg)
	if err != nil || index < 0 {
		return fmt.Errorf("invalid argument index %q for field %s", tags.arg, field.Name)
	}

	if err := validateSupportedFieldType(field); err != nil {
		return err
	}

	argName := bindingName(tags, parents)
	minCount, maxCount, err := argumentCounts(tags.nargs, field.Type)
	if err != nil {
		return fmt.Errorf("field %s: %w", field.Name, err)
	}

	for _, argument := range r.foundArguments {
		if argument.index == index {
			return fmt.Errorf("duplicate argument index: %d", index)
		}
	}

	r.foundArguments = append(r.foundArguments, &configArgument{
		index:        index,
		argumentName: argName,
		usageText:    tags.help,
		field:        field,
		fieldValue:   fieldValue,
		sources:      cli.NewValueSourceChain(valueSources...),
		defaultValue: tags.defaultValue,
		hasDefault:   tags.hasDefault,
		nargs:        tags.nargs,
		min:          minCount,
		max:          maxCount,
		info:         describeField(tags, parents),
		secret:       tags.isSecret,
	})

	return nil
}

func (r *structReflector) processFlag(field reflect.StructField, fieldValue reflect.Value, tags *configFieldTags, parents []*configFieldTags, valueSources []cli.ValueSource) error {
	flagName := bindingName(tags, parents)

	destination := fieldValue
	isPointer := field.Type.Kind() == reflect.Pointer
	hideDefault := isPointer && !tags.hasDefault
	if isPointer {
		field.Type = field.Type.Elem()
		fieldValue = reflect.New(field.Type).Elem()
	}

	sources := cli.NewValueSourceChain(valueSources...)

	var (
		flag  cli.Flag
		apply func(*cli.Command)
	)

	switch field.Type.Kind() { //nolint:exhaustive  // we have a default: clause that results in an error
	case reflect.String:
		flag = &cli.StringFlag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			HideDefault: hideDefault,
			Value:       tags.defaultValue,
			Sources:     sources,
			TakesFile:   tags.file,
		}

		apply = func(cmd *cli.Command) {
			fieldValue.SetString(cmd.String(flagName))
		}
	case reflect.Slice:
		repeated := tags.sliceMode == "repeated-flag"
		var defaults []string
		if tags.hasDefault {
			defaults = splitSliceValue(tags.defaultValue, repeated)
		}
		value, err := parseSlice(field, defaults)
		if err != nil {
			return fmt.Errorf("invalid default for %s: %w", field.Name, err)
		}

		slice := &sliceFlag{StringFlag: &cli.StringFlag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			Value:       tags.defaultValue,
			Sources:     sources,
		}, field: field, defaults: value, value: value, repeated: repeated, info: describeField(tags, parents), secret: tags.isSecret}
		flag = slice
		apply = func(_ *cli.Command) {
			fieldValue.Set(slice.value)
		}
	case reflect.Int:
		var value int
		if tags.defaultValue != "" {
			valueParsed, err := strconv.ParseInt(tags.defaultValue, 10, strconv.IntSize)
			if err != nil {
				return fmt.Errorf("failed to parse int value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}
			value = int(valueParsed)
		}

		flag = &cli.IntFlag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			HideDefault: hideDefault,
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetInt(int64(cmd.Int(flagName)))
		}
	case reflect.Int8:
		var value int8

		if tags.defaultValue != "" {
			valueParsed, err := strconv.ParseInt(tags.defaultValue, 10, 8)
			if err != nil {
				return fmt.Errorf("failed to parse int value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}

			value = int8(valueParsed)
		}

		flag = &cli.Int8Flag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			HideDefault: hideDefault,
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetInt(int64(cmd.Int8(flagName)))
		}
	case reflect.Int16:
		var value int16

		if tags.defaultValue != "" {
			valueParsed, err := strconv.ParseInt(tags.defaultValue, 10, 16)
			if err != nil {
				return fmt.Errorf("failed to parse int value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}

			value = int16(valueParsed)
		}

		flag = &cli.Int16Flag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			HideDefault: hideDefault,
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetInt(int64(cmd.Int16(flagName)))
		}
	case reflect.Int32:
		var value int32

		if tags.defaultValue != "" {
			valueParsed, err := strconv.ParseInt(tags.defaultValue, 10, 32)
			if err != nil {
				return fmt.Errorf("failed to parse int value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}

			value = int32(valueParsed)
		}

		flag = &cli.Int32Flag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			HideDefault: hideDefault,
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetInt(int64(cmd.Int32(flagName)))
		}
	case reflect.Int64:
		if _, ok := reflect.TypeAssert[time.Duration](fieldValue); ok { // special handling for time.Duration, which is a int64
			var (
				value time.Duration
				err   error
			)

			if tags.defaultValue != "" {
				value, err = time.ParseDuration(tags.defaultValue)
				if err != nil {
					return fmt.Errorf("failed to parse duration %s for field %s: %w", tags.defaultValue, field.Name, err)
				}
			}

			flag = &cli.DurationFlag{
				Name:        flagName,
				Aliases:     tags.aliases,
				Usage:       tags.help,
				DefaultText: tags.defaultValue,
				HideDefault: hideDefault,
				Value:       value,
				Sources:     sources,
			}
			apply = func(cmd *cli.Command) {
				fieldValue.SetInt(int64(cmd.Duration(flagName)))
			}
		} else {
			var (
				value int64
				err   error
			)

			if tags.defaultValue != "" {
				value, err = strconv.ParseInt(tags.defaultValue, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse int value %s for field %s: %w", tags.defaultValue, field.Name, err)
				}
			}

			flag = &cli.Int64Flag{
				Name:        flagName,
				Aliases:     tags.aliases,
				Usage:       tags.help,
				DefaultText: tags.defaultValue,
				HideDefault: hideDefault,
				Value:       value,
				Sources:     sources,
			}
			apply = func(cmd *cli.Command) {
				fieldValue.SetInt(cmd.Int64(flagName))
			}
		}
	case reflect.Uint:
		var (
			value uint64
			err   error
		)

		if tags.defaultValue != "" {
			value, err = strconv.ParseUint(tags.defaultValue, 10, strconv.IntSize)
			if err != nil {
				return fmt.Errorf("failed to parse uint value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}
		}

		flag = &cli.UintFlag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			HideDefault: hideDefault,
			Value:       uint(value),
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetUint(uint64(cmd.Uint(flagName)))
		}
	case reflect.Uint8:
		var value uint8

		if tags.defaultValue != "" {
			valueParsed, err := strconv.ParseUint(tags.defaultValue, 10, 8)
			if err != nil {
				return fmt.Errorf("failed to parse uint value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}

			value = uint8(valueParsed)
		}

		flag = &cli.Uint8Flag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			HideDefault: hideDefault,
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetUint(uint64(cmd.Uint8(flagName)))
		}
	case reflect.Uint16:
		var value uint16

		if tags.defaultValue != "" {
			valueParsed, err := strconv.ParseUint(tags.defaultValue, 10, 16)
			if err != nil {
				return fmt.Errorf("failed to parse uint value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}

			value = uint16(valueParsed)
		}

		flag = &cli.Uint16Flag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			HideDefault: hideDefault,
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetUint(uint64(cmd.Uint16(flagName)))
		}
	case reflect.Uint32:
		var value uint32

		if tags.defaultValue != "" {
			valueParsed, err := strconv.ParseUint(tags.defaultValue, 10, 32)
			if err != nil {
				return fmt.Errorf("failed to parse uint value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}

			value = uint32(valueParsed)
		}

		flag = &cli.Uint32Flag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			HideDefault: hideDefault,
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetUint(uint64(cmd.Uint32(flagName)))
		}
	case reflect.Uint64:
		var (
			value uint64
			err   error
		)

		if tags.defaultValue != "" {
			value, err = strconv.ParseUint(tags.defaultValue, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse uint value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}
		}

		flag = &cli.Uint64Flag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			HideDefault: hideDefault,
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetUint(cmd.Uint64(flagName))
		}
	case reflect.Float32:
		var value float32

		if tags.defaultValue != "" {
			valueParsed, err := strconv.ParseFloat(tags.defaultValue, 32)
			if err != nil {
				return fmt.Errorf("failed to parse float value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}

			value = float32(valueParsed)
		}

		flag = &cli.Float32Flag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			HideDefault: hideDefault,
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetFloat(float64(cmd.Float32(flagName)))
		}
	case reflect.Float64:
		var (
			value float64
			err   error
		)

		if tags.defaultValue != "" {
			value, err = strconv.ParseFloat(tags.defaultValue, 64)
			if err != nil {
				return fmt.Errorf("failed to parse float value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}
		}

		flag = &cli.Float64Flag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			HideDefault: hideDefault,
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetFloat(cmd.Float64(flagName))
		}
	case reflect.Bool:
		var (
			value bool
			err   error
		)

		if tags.defaultValue != "" {
			value, err = strconv.ParseBool(tags.defaultValue)
			if err != nil {
				return fmt.Errorf("failed to parse bool value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}
		}

		flag = &cli.BoolFlag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			HideDefault: hideDefault,
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetBool(cmd.Bool(flagName))
		}
	default:
		return fmt.Errorf("unknown field type %s", field.Type.Kind())
	}

	r.foundFlags = append(r.foundFlags, flag)
	r.applyFuncs = append(r.applyFuncs, apply)
	if isPointer {
		r.applyFuncs = append(r.applyFuncs, func(cmd *cli.Command) {
			destination.SetZero()
			if cmd.IsSet(flagName) || tags.hasDefault {
				destination.Set(reflect.New(field.Type))
				destination.Elem().Set(fieldValue)
			}
		})
	}

	return nil
}

func validateSupportedFieldType(field reflect.StructField) error {
	if field.Type.Kind() == reflect.Pointer || field.Type.Kind() == reflect.Slice {
		element := field.Type.Elem()
		if element.Kind() == reflect.Pointer || element.Kind() == reflect.Slice || element.Kind() == reflect.Struct {
			return fmt.Errorf("unsupported element type %s for field %s", element, field.Name)
		}
		field.Type = element
	}
	switch field.Type.Kind() { //nolint:exhaustive  // we have a default: clause that results in an error
	case reflect.String:
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return nil
	case reflect.Int64:
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return nil
	case reflect.Float32, reflect.Float64:
		return nil
	case reflect.Bool:
		return nil
	default:
		return fmt.Errorf("unknown field type %s", field.Type.Kind())
	}
}

func setFieldValueFromString(field reflect.StructField, fieldValue reflect.Value, value string) error {
	if field.Type.Kind() == reflect.Pointer {
		field.Type = field.Type.Elem()
		parsed := reflect.New(field.Type)
		if err := setFieldValueFromString(field, parsed.Elem(), value); err != nil {
			return err
		}
		fieldValue.Set(parsed)
		return nil
	}
	switch field.Type.Kind() { //nolint:exhaustive  // we have a default: clause that results in an error
	case reflect.String:
		fieldValue.SetString(value)
	case reflect.Slice:
		parsed, err := parseSlice(field, splitSliceValue(value, false))
		if err != nil {
			return err
		}
		fieldValue.Set(parsed)
	case reflect.Int:
		parsed, err := strconv.ParseInt(value, 10, strconv.IntSize)
		if err != nil {
			return fmt.Errorf("failed to parse int value %s for field %s: %w", value, field.Name, err)
		}
		fieldValue.SetInt(parsed)
	case reflect.Int8:
		parsed, err := strconv.ParseInt(value, 10, 8)
		if err != nil {
			return fmt.Errorf("failed to parse int value %s for field %s: %w", value, field.Name, err)
		}
		fieldValue.SetInt(parsed)
	case reflect.Int16:
		parsed, err := strconv.ParseInt(value, 10, 16)
		if err != nil {
			return fmt.Errorf("failed to parse int value %s for field %s: %w", value, field.Name, err)
		}
		fieldValue.SetInt(parsed)
	case reflect.Int32:
		parsed, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return fmt.Errorf("failed to parse int value %s for field %s: %w", value, field.Name, err)
		}
		fieldValue.SetInt(parsed)
	case reflect.Int64:
		if _, ok := reflect.TypeAssert[time.Duration](fieldValue); ok {
			parsed, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("failed to parse duration %s for field %s: %w", value, field.Name, err)
			}
			fieldValue.SetInt(int64(parsed))
			return nil
		}

		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse int value %s for field %s: %w", value, field.Name, err)
		}
		fieldValue.SetInt(parsed)
	case reflect.Uint:
		parsed, err := strconv.ParseUint(value, 10, strconv.IntSize)
		if err != nil {
			return fmt.Errorf("failed to parse uint value %s for field %s: %w", value, field.Name, err)
		}
		fieldValue.SetUint(parsed)
	case reflect.Uint8:
		parsed, err := strconv.ParseUint(value, 10, 8)
		if err != nil {
			return fmt.Errorf("failed to parse uint value %s for field %s: %w", value, field.Name, err)
		}
		fieldValue.SetUint(parsed)
	case reflect.Uint16:
		parsed, err := strconv.ParseUint(value, 10, 16)
		if err != nil {
			return fmt.Errorf("failed to parse uint value %s for field %s: %w", value, field.Name, err)
		}
		fieldValue.SetUint(parsed)
	case reflect.Uint32:
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			return fmt.Errorf("failed to parse uint value %s for field %s: %w", value, field.Name, err)
		}
		fieldValue.SetUint(parsed)
	case reflect.Uint64:
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("failed to parse uint value %s for field %s: %w", value, field.Name, err)
		}
		fieldValue.SetUint(parsed)
	case reflect.Float32:
		parsed, err := strconv.ParseFloat(value, 32)
		if err != nil {
			return fmt.Errorf("failed to parse float value %s for field %s: %w", value, field.Name, err)
		}
		fieldValue.SetFloat(parsed)
	case reflect.Float64:
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("failed to parse float value %s for field %s: %w", value, field.Name, err)
		}
		fieldValue.SetFloat(parsed)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("failed to parse bool value %s for field %s: %w", value, field.Name, err)
		}
		fieldValue.SetBool(parsed)
	default:
		return fmt.Errorf("unknown field type %s", field.Type.Kind())
	}

	return nil
}

func (r *structReflector) recurseStruct(anyStruct any, parents []*configFieldTags) error {
	structType := reflect.TypeOf(anyStruct)
	structValues := reflect.ValueOf(anyStruct)

	if structType.Kind() == reflect.Pointer {
		structType = structType.Elem()
		structValues = structValues.Elem()
	}

	for i := range structType.NumField() {
		fieldType := structType.Field(i)
		fieldValue := structValues.Field(i)
		if !fieldType.IsExported() {
			continue
		}

		tags := parseTagsWithFieldNameDefault(&fieldType.Tag, fieldType.Name)
		nested := slices.Clone(parents)
		nested = append(nested, tags)

		if fieldType.Type.Kind() == reflect.Struct {
			// recurse using the pointer to the nested struct, so we can modify it
			err := r.recurseStruct(fieldValue.Addr().Interface(), nested)
			if err != nil {
				return err
			}

			continue
		}

		if fieldType.Type.Kind() == reflect.Pointer && fieldType.Type.Elem().Kind() == reflect.Struct {
			if fieldValue.IsNil() {
				fieldValue.Set(reflect.New(fieldType.Type.Elem()))
			}

			err := r.recurseStruct(fieldValue.Interface(), nested)
			if err != nil {
				return err
			}

			continue
		}

		err := r.processField(fieldType, fieldValue, tags, parents)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *structReflector) validateArguments() error {
	if len(r.foundArguments) == 0 {
		return nil
	}

	seen := make(map[int]bool, len(r.foundArguments))
	for _, argument := range r.foundArguments {
		seen[argument.index] = true
	}

	for index := range len(r.foundArguments) {
		if !seen[index] {
			return fmt.Errorf("missing argument binding for index: %d", index)
		}
	}
	r.Arguments() // sort before checking positional grammar
	optional := false
	for i, argument := range r.foundArguments {
		if argument.max == -1 && i != len(r.foundArguments)-1 {
			return fmt.Errorf("variadic argument %s must be last", argument.argumentName)
		}
		if optional && argument.min > 0 {
			return fmt.Errorf("required argument %s cannot follow an optional argument", argument.argumentName)
		}
		optional = optional || argument.min == 0
	}

	return nil
}

func NewStructConfigurator(anyStruct any, tomlSources []cli.MapSource) (StructReflector, error) {
	return newStructConfigurator(anyStruct, tomlSources, commandOptions{})
}

func newStructConfigurator(anyStruct any, tomlSources []cli.MapSource, options commandOptions) (StructReflector, error) {
	v := reflect.ValueOf(anyStruct)
	if !v.IsValid() || v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return nil, errors.New("configuration must be a non-nil pointer to a struct")
	}
	_, err := Describe(anyStruct)
	if err != nil {
		return nil, err
	}
	reflector := &structReflector{
		foundFlags:     make([]cli.Flag, 0),
		foundArguments: make([]*configArgument, 0),
		applyFuncs:     make([]func(*cli.Command), 0),
		tomlSources:    tomlSources,
		options:        options,
	}

	err = reflector.recurseStruct(anyStruct, nil)
	if err != nil {
		return nil, err
	}

	if err := reflector.validateArguments(); err != nil {
		return nil, err
	}

	return reflector, nil
}

func argumentCounts(nargs string, t reflect.Type) (int, int, error) {
	minCount, maxCount := 0, 1
	switch nargs {
	case "":
		return minCount, maxCount, nil
	case "?":
		if t.Kind() == reflect.Slice {
			return 0, 0, errors.New("nargs ? requires a scalar")
		}
	case "*":
		maxCount = -1
	case "+":
		minCount, maxCount = 1, -1
	default:
		count, err := strconv.Atoi(nargs)
		if err != nil || count < 1 {
			return 0, 0, fmt.Errorf("invalid nargs %q", nargs)
		}
		minCount, maxCount = count, count
	}
	if maxCount != 1 && t.Kind() != reflect.Slice {
		return 0, 0, fmt.Errorf("nargs %s requires a slice", nargs)
	}
	return minCount, maxCount, nil
}
