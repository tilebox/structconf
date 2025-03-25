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
	Apply(*cli.Command)
}

type structReflector struct {
	foundFlags []cli.Flag           // flags found in the struct
	applyFuncs []func(*cli.Command) // functions to call after flags are parsed, to apply values to the struct

	tomlSources []cli.MapSource
}

func (r *structReflector) Flags() []cli.Flag {
	return r.foundFlags
}

func (r *structReflector) Apply(command *cli.Command) {
	for _, applyFunc := range r.applyFuncs {
		applyFunc(command)
	}
}

func (r *structReflector) processField(field reflect.StructField, fieldValue reflect.Value, tags *configFieldTags, parents []*configFieldTags) error {
	if tags == nil || tags.flag == "-" {
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

	if tags.env != "-" { // load from env var unless it's explicitly set to "-"
		envKey := tags.env
		if !tags.isGlobal {
			emvKeys := lo.Map(parents, func(parent *configFieldTags, _ int) string { return parent.env })
			emvKeys = append(emvKeys, tags.env)
			envKey = strings.Join(emvKeys, "_")
		}
		valueSources = append(valueSources, cli.EnvVar(envKey))
	}

	flagName := tags.flag
	if !tags.isGlobal {
		flagKeys := lo.Map(parents, func(parent *configFieldTags, _ int) string { return parent.flag })
		flagKeys = append(flagKeys, tags.flag)
		flagName = strings.Join(flagKeys, "-")
	}

	sources := cli.NewValueSourceChain(valueSources...)

	var flag cli.Flag
	var apply func(*cli.Command)

	switch field.Type.Kind() { //nolint:exhaustive
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
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		var value int64
		var err error
		if tags.defaultValue != "" {
			value, err = strconv.ParseInt(tags.defaultValue, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse int value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}
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
			fieldValue.SetInt(cmd.Int(flagName))
		}
	case reflect.Int64:
		if _, ok := fieldValue.Interface().(time.Duration); ok { // special handling for time.Duration, which is a int64
			var value time.Duration
			var err error
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
			var value int64
			var err error
			if tags.defaultValue != "" {
				value, err = strconv.ParseInt(tags.defaultValue, 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse int value %s for field %s: %w", tags.defaultValue, field.Name, err)
				}
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
				fieldValue.SetInt(cmd.Int(flagName))
			}
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		var value uint64
		var err error
		if tags.defaultValue != "" {
			value, err = strconv.ParseUint(tags.defaultValue, 10, 64)
			if err != nil {
				return fmt.Errorf("failed to parse uint value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}
		}

		flag = &cli.UintFlag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetUint(cmd.Uint(flagName))
		}
	case reflect.Float32, reflect.Float64:
		var value float64
		var err error
		if tags.defaultValue != "" {
			value, err = strconv.ParseFloat(tags.defaultValue, 64)
			if err != nil {
				return fmt.Errorf("failed to parse float value %s for field %s: %w", tags.defaultValue, field.Name, err)
			}
		}

		flag = &cli.FloatFlag{
			Name:        flagName,
			Aliases:     tags.aliases,
			Usage:       tags.help,
			DefaultText: tags.defaultValue,
			Value:       value,
			Sources:     sources,
		}
		apply = func(cmd *cli.Command) {
			fieldValue.SetFloat(cmd.Float(flagName))
		}
	case reflect.Bool:
		var value bool
		var err error
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

func (r *structReflector) recurseStruct(anyStruct any, parents []*configFieldTags) error {
	structType := reflect.TypeOf(anyStruct)
	structValues := reflect.ValueOf(anyStruct)

	if structType.Kind() == reflect.Ptr {
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

		if fieldType.Type.Kind() == reflect.Ptr && fieldType.Type.Elem().Kind() == reflect.Struct {
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

func NewStructConfigurator(anyStruct any, tomlSources []cli.MapSource) (StructReflector, error) {
	reflector := &structReflector{
		foundFlags:  make([]cli.Flag, 0),
		applyFuncs:  make([]func(*cli.Command), 0),
		tomlSources: tomlSources,
	}

	err := reflector.recurseStruct(anyStruct, nil)
	if err != nil {
		return nil, err
	}
	return reflector, nil
}
