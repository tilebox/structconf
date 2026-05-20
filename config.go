package structconf

import (
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
}

func (a *configArgument) HasName(name string) bool {
	return name == a.argumentName
}

func (a *configArgument) Parse(args []string) ([]string, error) {
	if len(args) > 0 {
		if err := setFieldValueFromString(a.field, a.fieldValue, args[0]); err != nil {
			return args, fmt.Errorf("invalid value %q for argument %s: %w", args[0], a.argumentName, err)
		}

		return args[1:], nil
	}

	if value, found := a.sources.Lookup(); found {
		if err := setFieldValueFromString(a.field, a.fieldValue, value); err != nil {
			return args, fmt.Errorf("failed to parse source value %s for argument %s: %w", value, a.argumentName, err)
		}

		return args, nil
	}

	if a.defaultValue != "" {
		if err := setFieldValueFromString(a.field, a.fieldValue, a.defaultValue); err != nil {
			return args, fmt.Errorf("failed to parse default value %s for argument %s: %w", a.defaultValue, a.argumentName, err)
		}
	}

	return args, nil
}

func (a *configArgument) Usage() string {
	return a.argumentName
}

func (a *configArgument) Get() any {
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

func typeName(valueType reflect.Type) string {
	return valueType.String()
}

func (r *structReflector) processField(field reflect.StructField, fieldValue reflect.Value, tags *configFieldTags, parents []*configFieldTags) error {
	if tags == nil {
		return nil
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

	if tags.env != "-" { // load from env var unless it's explicitly set to "-"
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

	argName := tags.toml
	if !tags.isGlobal {
		argKeys := lo.Map(parents, func(parent *configFieldTags, _ int) string { return parent.flag })
		argKeys = append(argKeys, tags.toml)
		argName = strings.Join(argKeys, "-")
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
	})

	return nil
}

func (r *structReflector) processFlag(field reflect.StructField, fieldValue reflect.Value, tags *configFieldTags, parents []*configFieldTags, valueSources []cli.ValueSource) error {
	flagName := tags.flag
	if !tags.isGlobal {
		flagKeys := lo.Map(parents, func(parent *configFieldTags, _ int) string { return parent.flag })
		flagKeys = append(flagKeys, tags.flag)
		flagName = strings.Join(flagKeys, "-")
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
			Value:       tags.defaultValue,
			Sources:     sources,
		}

		apply = func(cmd *cli.Command) {
			fieldValue.SetString(cmd.String(flagName))
		}
	case reflect.Slice:
		if field.Type.Elem().Kind() != reflect.String {
			return fmt.Errorf("unsupported slice element type %s for field %s", field.Type.Elem().Kind(), field.Name)
		}

		flag = &cli.StringFlag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			Value:       tags.defaultValue,
			Sources:     sources,
		}

		apply = func(cmd *cli.Command) {
			value := cmd.String(flagName)
			if value == "" {
				fieldValue.Set(reflect.Zero(field.Type))
				return
			}

			fieldValue.Set(reflect.ValueOf(strings.Split(value, ",")))
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
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetInt(int64(cmd.Int32(flagName)))
		}
	case reflect.Int64:
		if _, ok := fieldValue.Interface().(time.Duration); ok { // special handling for time.Duration, which is a int64
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

	return nil
}

func validateSupportedFieldType(field reflect.StructField) error {
	switch field.Type.Kind() { //nolint:exhaustive  // we have a default: clause that results in an error
	case reflect.String:
		return nil
	case reflect.Slice:
		if field.Type.Elem().Kind() != reflect.String {
			return fmt.Errorf("unsupported slice element type %s for field %s", field.Type.Elem().Kind(), field.Name)
		}
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
	switch field.Type.Kind() { //nolint:exhaustive  // we have a default: clause that results in an error
	case reflect.String:
		fieldValue.SetString(value)
	case reflect.Slice:
		if field.Type.Elem().Kind() != reflect.String {
			return fmt.Errorf("unsupported slice element type %s for field %s", field.Type.Elem().Kind(), field.Name)
		}

		if value == "" {
			fieldValue.Set(reflect.Zero(field.Type))
			return nil
		}

		fieldValue.Set(reflect.ValueOf(strings.Split(value, ",")))
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
		if _, ok := fieldValue.Interface().(time.Duration); ok {
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

	return nil
}

func NewStructConfigurator(anyStruct any, tomlSources []cli.MapSource) (StructReflector, error) {
	reflector := &structReflector{
		foundFlags:     make([]cli.Flag, 0),
		foundArguments: make([]*configArgument, 0),
		applyFuncs:     make([]func(*cli.Command), 0),
		tomlSources:    tomlSources,
	}

	err := reflector.recurseStruct(anyStruct, nil)
	if err != nil {
		return nil, err
	}

	if err := reflector.validateArguments(); err != nil {
		return nil, err
	}

	return reflector, nil
}
