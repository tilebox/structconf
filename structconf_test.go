package structconf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func Test_LoadArgsFullyTagged(t *testing.T) {
	type config struct {
		Value  string `flag:"value" env:"VALUE" default:"value-from-default-tag" toml:"value"`
		Nested struct {
			Value string `flag:"value" env:"VALUE" default:"nested-value-from-default-tag" toml:"value"`
		} `flag:"nested" env:"NESTED" toml:"nested"`
		Doubly struct {
			Nested struct {
				Value string `flag:"value" env:"VALUE" default:"double-nested-value-from-default-tag" toml:"value"`
			} `flag:"nested" env:"NESTED" toml:"nested"`
		} `flag:"doubly"   env:"DOUBLY"   toml:"doubly"`
		Duration time.Duration `flag:"duration" env:"DURATION" default:"10s" toml:"duration"`
	}

	type args struct {
		cliArgs []string
	}

	tests := []struct {
		name                  string
		args                  *args
		wantValue             string
		wantNestedValue       string
		wantDoublyNestedValue string
		wantDuration          time.Duration
	}{
		{
			name: "parse flags",
			args: &args{
				cliArgs: []string{"my-program", "--value", "value-from-cli", "--nested-value", "nested-value-from-cli", "--doubly-nested-value", "doubly-nested-value-from-cli", "--duration", "2s"},
			},
			wantValue:             "value-from-cli",
			wantNestedValue:       "nested-value-from-cli",
			wantDoublyNestedValue: "doubly-nested-value-from-cli",
			wantDuration:          2 * time.Second,
		},
	}

	_ = tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config{}

			SetArgsForTest(t, tt.args.cliArgs) // set cli args, and clean up after the test

			err := LoadArgs(config, "my-program", os.Args, WithDefaultLoadConfigFlag())
			require.NoError(t, err)

			assert.Equal(t, tt.wantValue, config.Value)
			assert.Equal(t, tt.wantNestedValue, config.Nested.Value)
			assert.Equal(t, tt.wantDoublyNestedValue, config.Doubly.Nested.Value)
			assert.Equal(t, tt.wantDuration, config.Duration)
		})
	}
}

func Test_LoadArgsDefaultTags(t *testing.T) {
	type config struct {
		Value  string
		Nested struct {
			Value string
		}
		Doubly struct {
			Nested struct {
				Value string
			}
		}
		Duration time.Duration
	}

	type args struct {
		cliArgs []string
	}

	tests := []struct {
		name                  string
		args                  *args
		wantValue             string
		wantNestedValue       string
		wantDoublyNestedValue string
		wantDuration          time.Duration
	}{
		{
			name: "parse flags using default tags",
			args: &args{
				cliArgs: []string{"my-program", "--value", "value-from-cli", "--nested-value", "nested-value-from-cli", "--doubly-nested-value", "doubly-nested-value-from-cli", "--duration", "2s"},
			},
			wantValue:             "value-from-cli",
			wantNestedValue:       "nested-value-from-cli",
			wantDoublyNestedValue: "doubly-nested-value-from-cli",
			wantDuration:          2 * time.Second,
		},
	}

	_ = tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config{}

			SetArgsForTest(t, tt.args.cliArgs) // set cli args, and clean up after the test

			err := LoadArgs(config, "my-program", os.Args, WithDefaultLoadConfigFlag())
			require.NoError(t, err)

			assert.Equal(t, tt.wantValue, config.Value)
			assert.Equal(t, tt.wantNestedValue, config.Nested.Value)
			assert.Equal(t, tt.wantDoublyNestedValue, config.Doubly.Nested.Value)
			assert.Equal(t, tt.wantDuration, config.Duration)
		})
	}
}

func Test_LoadArgsStringSlice(t *testing.T) {
	type config struct {
		Values []string `flag:"values" env:"VALUES" default:"one,two"`
	}

	tests := []struct {
		name     string
		cliArgs  []string
		envValue string
		toml     string
		want     []string
	}{
		{
			name:    "parse comma-separated flag value",
			cliArgs: []string{"my-program", "--values", "three,four"},
			want:    []string{"three", "four"},
		},
		{
			name:     "parse comma-separated env value",
			cliArgs:  []string{"my-program"},
			envValue: "five,six",
			want:     []string{"five", "six"},
		},
		{
			name:    "parse toml string array value",
			cliArgs: []string{"my-program"},
			toml:    `values = ["seven", "eight"]`,
			want:    []string{"seven", "eight"},
		},
		{
			name:    "parse comma-separated default value",
			cliArgs: []string{"my-program"},
			want:    []string{"one", "two"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config{}
			cliArgs := slices.Clone(tt.cliArgs)
			if tt.toml != "" {
				configPath := path.Join(t.TempDir(), "test-config.toml")
				require.NoError(t, os.WriteFile(configPath, []byte(tt.toml), 0o600))
				cliArgs = append(cliArgs, "--load-config", configPath)
			}
			SetArgsForTest(t, cliArgs)
			if tt.envValue != "" {
				t.Setenv("VALUES", tt.envValue)
			}

			err := LoadArgs(cfg, "my-program", os.Args, WithDefaultLoadConfigFlag())
			require.NoError(t, err)

			assert.Equal(t, tt.want, cfg.Values)
		})
	}
}

func TestSliceModesAndSources(t *testing.T) {
	type config struct {
		Filters []string `flag:"filter" env:"FILTERS" default:"default,filter" toml:"filters" slice:"repeated-flag"`
		Counts  []int    `env:"COUNTS"  default:"3,7" slice:"comma-separated"`
	}
	for _, tc := range []struct {
		name    string
		args    []string
		toml    string
		env     bool
		filters []string
		counts  []int
	}{
		{"defaults", nil, "", false, []string{"default,filter"}, []int{3, 7}},
		{"environment", nil, "", true, []string{"env,filter"}, []int{11, 13}},
		{"toml overrides env", nil, `filters = ["a,b", "c"]` + "\ncounts = [17, 19]", true, []string{"a,b", "c"}, []int{17, 19}},
		{"cli replaces fallbacks", []string{"--filter", "x,y", "--filter", "", "--counts", "23,29", "--counts", "31,37"}, `filters = ["toml"]` + "\ncounts = [41]", true, []string{"x,y", ""}, []int{31, 37}},
		{"empty arrays override", nil, "filters = []\ncounts = []", true, nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env {
				t.Setenv("FILTERS", "env,filter")
				t.Setenv("COUNTS", "11,13")
			}
			args := append([]string{"app"}, tc.args...)
			if tc.toml != "" {
				file := path.Join(t.TempDir(), "config.toml")
				require.NoError(t, os.WriteFile(file, []byte(tc.toml), 0o600))
				args = append(args, "--load-config", file)
			}
			var cfg config
			require.NoError(t, LoadArgs(&cfg, "app", args, WithDefaultLoadConfigFlag()))
			assert.Equal(t, tc.filters, cfg.Filters)
			assert.Equal(t, tc.counts, cfg.Counts)
		})
	}
}

func TestPrimitiveSlices(t *testing.T) {
	for _, tc := range []struct {
		want    any
		values  []string
		invalid string
	}{
		{[]string{"a,b", " c "}, []string{"a,b", " c "}, ""},
		{[]int{-3, 17}, []string{"-3", "17"}, "x"},
		{[]int8{-128, 127}, []string{"-128", "127"}, "128"},
		{[]int16{-32768, 32767}, []string{"-32768", "32767"}, "32768"},
		{[]int32{-2147483648, 2147483647}, []string{"-2147483648", "2147483647"}, "2147483648"},
		{[]int64{-9223372036854775808, 9223372036854775807}, []string{"-9223372036854775808", "9223372036854775807"}, "9223372036854775808"},
		{[]uint{0, 17}, []string{"0", "17"}, "-1"},
		{[]uint8{0, 255}, []string{"0", "255"}, "256"},
		{[]uint16{0, 65535}, []string{"0", "65535"}, "65536"},
		{[]uint32{0, 4294967295}, []string{"0", "4294967295"}, "4294967296"},
		{[]uint64{0, 18446744073709551615}, []string{"0", "18446744073709551615"}, "18446744073709551616"},
		{[]float32{-1.25, 2.5}, []string{"-1.25", "2.5"}, "1e100"},
		{[]float64{-1.25, 2.5}, []string{"-1.25", "2.5"}, "1e1000"},
		{[]bool{false, true}, []string{"false", "true"}, "maybe"},
		{[]time.Duration{-time.Second, 1500 * time.Millisecond}, []string{"-1s", "1.5s"}, "1day"},
	} {
		t.Run(reflect.TypeOf(tc.want).String(), func(t *testing.T) {
			// Every supported scalar also binds through a pointer, including named
			// types such as time.Duration. Omission must not allocate it.
			element := reflect.TypeOf(tc.want).Elem()
			for _, binding := range []string{`flag:"value"`, `arg:"0"`} {
				pointerType := reflect.StructOf([]reflect.StructField{{Name: "Value", Type: reflect.PointerTo(element), Tag: reflect.StructTag(binding + ` env:"-"`)}})
				cfg := reflect.New(pointerType)
				require.NoError(t, LoadArgs(cfg.Interface(), "app", []string{"app"}))
				assert.True(t, cfg.Elem().Field(0).IsNil())
				args := []string{"app"}
				if strings.HasPrefix(binding, "flag") {
					args = append(args, "--value="+tc.values[0])
				} else {
					args = append(args, "--", tc.values[0])
				}
				require.NoError(t, LoadArgs(cfg.Interface(), "app", args))
				require.False(t, cfg.Elem().Field(0).IsNil())
				assert.Equal(t, reflect.ValueOf(tc.want).Index(0).Interface(), cfg.Elem().Field(0).Elem().Interface())
			}
			for _, mode := range []string{"comma-separated", "repeated-flag"} {
				if _, ok := tc.want.([]string); ok && mode == "comma-separated" {
					continue
				}
				typeOf := reflect.StructOf([]reflect.StructField{{Name: "Values", Type: reflect.TypeOf(tc.want), Tag: reflect.StructTag(`env:"-" slice:"` + mode + `"`)}})
				cfg := reflect.New(typeOf)
				args := []string{"app"}
				if mode == "comma-separated" {
					args = append(args, "--values", strings.Join(tc.values, ","))
				} else {
					for _, value := range tc.values {
						args = append(args, "--values", value)
					}
				}
				require.NoError(t, LoadArgs(cfg.Interface(), "app", args))
				assert.Equal(t, tc.want, cfg.Elem().Field(0).Interface())
				if tc.invalid != "" {
					require.Error(t, LoadArgs(reflect.New(typeOf).Interface(), "app", []string{"app", "--values", tc.invalid}))
				}
			}
		})
	}
}

func TestOptionalScalars(t *testing.T) {
	type config struct {
		Name    *string        `arg:"0"     env:"-"       nargs:"?"`
		Note    *string        `env:"-"`
		Enabled *bool          `env:"-"`
		Count   *int           `env:"-"`
		Wait    *time.Duration `env:"-"`
		Default *string        `env:"-"     default:""`
		Token   *string        `env:"TOKEN" secret:"true"`
	}
	var absent config
	require.NoError(t, LoadArgs(&absent, "app", []string{"app"}, WithDisableImplicitEnvVars()))
	assert.Nil(t, absent.Name)
	assert.Nil(t, absent.Note)
	assert.Nil(t, absent.Enabled)
	assert.Nil(t, absent.Count)
	assert.Nil(t, absent.Wait)
	require.NotNil(t, absent.Default)
	assert.Empty(t, *absent.Default)
	values, err := MarshalAsMap(&absent)
	require.NoError(t, err)
	assert.NotContains(t, values, "count")
	assert.Contains(t, values, "default")
	t.Setenv("TOKEN", "private-secret")
	var cfg config
	require.NoError(t, LoadArgs(&cfg, "app", []string{"app", "--note=", "--enabled=false", "--count=0", "--wait=0s", ""}))
	require.NotNil(t, cfg.Name)
	assert.Empty(t, *cfg.Name)
	require.NotNil(t, cfg.Note)
	assert.Empty(t, *cfg.Note)
	require.NotNil(t, cfg.Enabled)
	assert.False(t, *cfg.Enabled)
	require.NotNil(t, cfg.Count)
	assert.Zero(t, *cfg.Count)
	require.NotNil(t, cfg.Wait)
	assert.Zero(t, *cfg.Wait)
	values, err = MarshalAsMap(&cfg)
	require.NoError(t, err)
	assert.Equal(t, "pr***et", values["token"])
	assert.Equal(t, false, values["enabled"])
	assert.Equal(t, int64(0), values["count"])
	file := path.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(file, []byte("count = 17\nenabled = false\nnote = ''\n"), 0o600))
	require.NoError(t, LoadArgs(&cfg, "app", []string{"app", "--load-config", file}, WithDefaultLoadConfigFlag()))
	require.NotNil(t, cfg.Count)
	assert.Equal(t, 17, *cfg.Count)
	require.NotNil(t, cfg.Enabled)
	assert.False(t, *cfg.Enabled)
	require.NotNil(t, cfg.Note)
	assert.Empty(t, *cfg.Note)
	assert.Nil(t, cfg.Name)
}

func TestPointerArgumentGetters(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		wantName  string
		wantCount int
	}{
		{"omitted", nil, "", 0},
		{"provided", []string{"hello", "42"}, "hello", 42},
		{"explicit zero", []string{"", "0"}, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &struct {
				Name  *string `arg:"0" env:"-" nargs:"?"`
				Count *int    `arg:"1" env:"-" nargs:"?"`
			}{Name: new("previous"), Count: new(99)}
			cmd, err := NewCommand(cfg, "app", func(_ context.Context, cmd *cli.Command) error {
				assert.Equal(t, tc.wantName, cmd.StringArg("name"))
				assert.Equal(t, tc.wantCount, cmd.IntArg("count"))
				return nil
			})
			require.NoError(t, err)
			assert.Equal(t, "string", cmd.Arguments[0].(ArgumentMetadata).TypeName())
			assert.Equal(t, "int", cmd.Arguments[1].(ArgumentMetadata).TypeName())
			require.NoError(t, cmd.Run(t.Context(), append([]string{"app"}, tc.args...)))
			if len(tc.args) == 0 {
				assert.Nil(t, cfg.Name)
				assert.Nil(t, cfg.Count)
			} else {
				require.NotNil(t, cfg.Name)
				require.NotNil(t, cfg.Count)
				assert.Equal(t, tc.wantName, *cfg.Name)
				assert.Equal(t, tc.wantCount, *cfg.Count)
			}
		})
	}
}

func TestOptionalFlagMetadata(t *testing.T) {
	for _, zero := range []any{"", false, int(0), int8(0), int16(0), int32(0), int64(0), uint(0), uint8(0), uint16(0), uint32(0), uint64(0), float32(0), float64(0), time.Duration(0)} {
		t.Run(reflect.TypeOf(zero).String(), func(t *testing.T) {
			scalar := reflect.TypeOf(zero)
			defaultText := fmt.Sprint(zero)
			typ := reflect.StructOf([]reflect.StructField{
				{Name: "Omitted", Type: reflect.PointerTo(scalar), Tag: `env:"-"`},
				{Name: "Default", Type: reflect.PointerTo(scalar), Tag: reflect.StructTag(`env:"-" default:"` + defaultText + `"`)},
				{Name: "Scalar", Type: scalar, Tag: `env:"-"`},
			})
			cfg := reflect.New(typ)
			cmd, err := NewCommand(cfg.Interface(), "app", nil)
			require.NoError(t, err)
			omitted := cmd.Flags[0].(cli.DocGenerationFlag)
			assert.False(t, omitted.IsDefaultVisible())
			assert.Empty(t, omitted.GetDefaultText())
			assert.NotContains(t, cmd.Flags[0].String(), "default:")
			explicit := cmd.Flags[1].(cli.DocGenerationFlag)
			assert.True(t, explicit.IsDefaultVisible())
			assert.Equal(t, defaultText, explicit.GetDefaultText())
			assert.True(t, cmd.Flags[2].(cli.DocGenerationFlag).IsDefaultVisible())
			require.NoError(t, cmd.Run(t.Context(), []string{"app"}))
			assert.True(t, cfg.Elem().Field(0).IsNil())
			require.False(t, cfg.Elem().Field(1).IsNil())
			assert.Equal(t, zero, cfg.Elem().Field(1).Elem().Interface())
		})
	}
}

func TestLegacyArgumentInitialValues(t *testing.T) {
	for _, tc := range []struct {
		name       string
		args       []string
		env        bool
		wantName   string
		wantValues []string
	}{
		{"omitted", nil, false, "initial", []string{"initial", "values"}},
		{"provided", []string{"cli", "a,b"}, false, "cli", []string{"a", "b"}},
		{"explicit empty", []string{"", ""}, false, "", nil},
		{"source", nil, true, "env", []string{"source", "values"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &struct {
				Name     string   `arg:"0" env:"INITIAL_NAME"`
				Values   []string `arg:"1" env:"INITIAL_VALUES"`
				Optional string   `arg:"2" env:"-"              nargs:"?"`
				Rest     []string `arg:"3" env:"-"              nargs:"*"`
			}{Name: "initial", Values: []string{"initial", "values"}, Optional: "previous", Rest: []string{"previous"}}
			if tc.env {
				t.Setenv("INITIAL_NAME", "env")
				t.Setenv("INITIAL_VALUES", "source,values")
			}
			require.NoError(t, LoadArgs(cfg, "app", append([]string{"app"}, tc.args...)))
			assert.Equal(t, tc.wantName, cfg.Name)
			assert.Equal(t, tc.wantValues, cfg.Values)
			assert.Empty(t, cfg.Optional)
			assert.Nil(t, cfg.Rest)
		})
	}
}

func TestArgumentCardinality(t *testing.T) {
	for _, tc := range []struct {
		nargs string
		args  []string
		want  []string
		fail  bool
	}{
		{"", []string{"a,b"}, []string{"a", "b"}, false},
		{"", []string{"a", "extra"}, nil, true},
		{"1", []string{"a,b"}, []string{"a,b"}, false},
		{"1", nil, nil, true},
		{"2", []string{"a,b", "c"}, []string{"a,b", "c"}, false},
		{"2", []string{"a"}, nil, true},
		{"2", []string{"a", "b", "extra"}, nil, true},
		{"*", nil, nil, false},
		{"*", []string{"a,b", "", "c"}, []string{"a,b", "", "c"}, false},
		{"+", nil, nil, true},
		{"+", []string{""}, []string{""}, false},
	} {
		t.Run(tc.nargs+strings.Join(tc.args, "/"), func(t *testing.T) {
			typeOf := reflect.StructOf([]reflect.StructField{{Name: "Values", Type: reflect.TypeFor[[]string](), Tag: reflect.StructTag(`arg:"0" env:"-" nargs:"` + tc.nargs + `"`)}})
			cfg := reflect.New(typeOf)
			err := LoadArgs(cfg.Interface(), "app", append([]string{"app"}, tc.args...))
			if tc.fail {
				var validation *ValidationError
				require.ErrorAs(t, err, &validation)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, cfg.Elem().Field(0).Interface())
		})
	}
	var cfg struct {
		Pair   []int  `arg:"0" env:"-" nargs:"2"`
		Output string `arg:"1" env:"-" nargs:"1"`
	}
	require.NoError(t, LoadArgs(&cfg, "app", []string{"app", "3", "17", "output"}))
	assert.Equal(t, []int{3, 17}, cfg.Pair)
	assert.Equal(t, "output", cfg.Output)
	err := LoadArgs(&cfg, "app", []string{"app", "3", "17", "output", "extra"}, WithDefaultLoadConfigFlag())
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Contains(t, err.Error(), "1 extra")
	assert.NotContains(t, err.Error(), "output")
	var flagsOnly struct{ Name string }
	require.NoError(t, LoadArgs(&flagsOnly, "app", []string{"app", "unbound"}))
}

func TestArgumentSourcesAndPassthrough(t *testing.T) {
	type config struct {
		Pair []int `arg:"0" env:"PAIR" default:"3,7" nargs:"2"`
	}
	t.Setenv("PAIR", "11,17")
	for _, tc := range []struct {
		args []string
		want []int
		fail bool
	}{
		{nil, []int{11, 17}, false},
		{[]string{"23"}, nil, true},
		{[]string{"23", "29"}, []int{23, 29}, false},
	} {
		var cfg config
		err := LoadArgs(&cfg, "app", append([]string{"app"}, tc.args...))
		if tc.fail {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		assert.Equal(t, tc.want, cfg.Pair)
	}
	file := path.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(file, []byte("pair = [31, 37]"), 0o600))
	var cfg config
	require.NoError(t, LoadArgs(&cfg, "app", []string{"app", "--load-config", file}, WithDefaultLoadConfigFlag()))
	assert.Equal(t, []int{31, 37}, cfg.Pair)
	var pass struct {
		Command []string `arg:"0" env:"-" nargs:"+"`
	}
	cmd, err := NewCommand(&pass, "parallel", nil)
	require.NoError(t, err)
	cmd.StopOnNthArg = new(1)
	require.NoError(t, cmd.Run(t.Context(), []string{"parallel", "--", "echo", "--not-a-tilebox-flag", "a,b", ""}))
	assert.Equal(t, []string{"echo", "--not-a-tilebox-flag", "a,b", ""}, pass.Command)
	var rest []string
	cmd, err = NewCommand(&cfg, "app", func(_ context.Context, cmd *cli.Command) error {
		rest = cmd.Args().Slice()
		return nil
	})
	require.NoError(t, err)
	require.NoError(t, cmd.Run(t.Context(), []string{"app", "3", "17", "forwarded"}))
	assert.Equal(t, []int{3, 17}, cfg.Pair)
	assert.Equal(t, []string{"forwarded"}, rest)
}

func TestRepeatedFlagInheritance(t *testing.T) {
	type config struct {
		Values []string `env:"VALUES" slice:"repeated-flag"`
	}
	t.Setenv("VALUES", "fallback,one")
	for _, tc := range []struct {
		name  string
		args  []string
		want  []string
		count int
	}{
		{"fallback", []string{"app", "child"}, []string{"fallback,one"}, 1},
		{"child replaces fallback", []string{"app", "child", "--values", "a,b", "--values", "c"}, []string{"a,b", "c"}, 2},
		{"parent and child", []string{"app", "--values", "parent", "child", "--values", "child"}, []string{"parent", "child"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var cfg config
			root, err := NewCommand(&cfg, "app", nil)
			require.NoError(t, err)
			var got []string
			var count int
			root.Commands = []*cli.Command{{Name: "child", Action: func(_ context.Context, cmd *cli.Command) error {
				got, count = cmd.StringSlice("values"), cmd.Count("values")
				return nil
			}}}
			require.NoError(t, root.Run(t.Context(), tc.args))
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.count, count)
		})
	}
}

func TestSliceFlagInterfaces(t *testing.T) {
	var cfg struct {
		Values []int `env:"-" default:"3,7"`
	}
	var count int
	cmd, err := NewCommand(&cfg, "app", func(_ context.Context, cmd *cli.Command) error {
		count = cmd.Count("values")
		return nil
	})
	require.NoError(t, err)
	flag := cmd.Flags[0]
	stringer := flag.(cli.StringerSetter)
	original := flag.String()
	stringer.SetStringer(func(f cli.Flag) string { return "custom " + f.Names()[0] })
	assert.Equal(t, "custom values", flag.String())
	stringer.SetStringer(nil)
	assert.Equal(t, original, flag.String())
	require.NoError(t, cmd.Run(t.Context(), []string{"app", "--values", "11,13", "--values", "17,19"}))
	assert.Equal(t, []int{17, 19}, cfg.Values)
	assert.Equal(t, 2, count)
}

func TestInvalidBindingTags(t *testing.T) {
	for _, cfg := range []any{
		&struct {
			X []string `slice:"csv"`
		}{},
		&struct {
			X int `slice:"repeated-flag"`
		}{},
		&struct {
			X []string `arg:"0" slice:"repeated-flag"`
		}{},
		&struct {
			X string `nargs:"1"`
		}{},
		&struct {
			X []string `arg:"0" nargs:"0"`
		}{},
		&struct {
			X []string `arg:"0" nargs:"-1"`
		}{},
		&struct {
			X string `arg:"0" nargs:"2"`
		}{},
		&struct {
			X []string `arg:"0" nargs:"?"`
		}{},
		&struct {
			X int `file:"true"`
		}{},
		&struct {
			X string `file:"yes"`
		}{},
		&struct {
			X []string `arg:"0" nargs:"*"`
			Y string   `arg:"1"`
		}{},
		&struct {
			X *string `arg:"0" nargs:"?"`
			Y string  `arg:"1" nargs:"1"`
		}{},
		&struct {
			X string `arg:"0" validate:"required"`
			Y string `arg:"1" nargs:"1"`
		}{},
		&struct{ X [][]string }{},
		&struct{ X **string }{},
	} {
		_, err := NewCommand(cfg, "app", nil)
		require.Error(t, err, "%T", cfg)
	}
}

func TestExplicitEnvironmentAndFileCompletion(t *testing.T) {
	type config struct {
		Implicit   string
		Empty      string  `env:""`
		Explicit   string  `env:"BOUND"`
		File       *string `env:"-"     file:"true"`
		Positional *string `arg:"0"`
	}
	t.Setenv("IMPLICIT", "implicit")
	t.Setenv("EMPTY", "empty")
	t.Setenv("BOUND", "explicit")
	t.Setenv("POSITIONAL", "positional")
	var cfg config
	require.NoError(t, LoadArgs(&cfg, "app", []string{"app"}))
	assert.Equal(t, "implicit", cfg.Implicit)
	require.NotNil(t, cfg.Positional)
	for _, loadConfig := range []bool{false, true} {
		var cfg config
		opts := []Option{WithDisableImplicitEnvVars()}
		if loadConfig {
			opts = append(opts, WithDefaultLoadConfigFlag())
		}
		require.NoError(t, LoadArgs(&cfg, "app", []string{"app"}, opts...))
		assert.Empty(t, cfg.Implicit)
		assert.Empty(t, cfg.Empty)
		assert.Nil(t, cfg.Positional)
		assert.Equal(t, "explicit", cfg.Explicit)
	}
	var bound config
	cmd, err := NewCommand(&bound, "app", nil, WithDisableImplicitEnvVars())
	require.NoError(t, err)
	var file *cli.StringFlag
	for _, flag := range cmd.Flags {
		if flag.Names()[0] == "file" {
			file = flag.(*cli.StringFlag)
		}
	}
	require.NotNil(t, file)
	assert.True(t, file.TakesFile)
	require.NoError(t, cmd.Run(t.Context(), []string{"app", "--file", "input.txt"}))
	assert.Empty(t, bound.Implicit)
	assert.Equal(t, "explicit", bound.Explicit)
	require.NotNil(t, bound.File)
	assert.Equal(t, "input.txt", *bound.File)
}

func TestStructuredValidation(t *testing.T) {
	type nested struct {
		Token string `flag:"credential" secret:"true" validate:"oneof=allowed" help:"authentication token"`
	}
	cfg := &struct {
		Nested *nested `flag:"auth"`
	}{}
	fields, err := Describe(cfg)
	require.NoError(t, err)
	assert.Nil(t, cfg.Nested)
	assert.Equal(t, []FieldInfo{{Path: "Nested.Token", Name: "--auth-credential", Help: "authentication token"}}, fields)
	cfg.Nested = &nested{Token: "private-secret"}
	err = Validate(cfg)
	var validation *ValidationError
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, fields, validation.Issues[0].Fields)
	assert.Equal(t, "oneof", validation.Issues[0].Rule)
	assert.Equal(t, "allowed", validation.Issues[0].Param)
	assert.Equal(t, "--auth-credential must be one of: allowed", validation.Issues[0].Message)
	assert.NotContains(t, err.Error(), "private-secret")
	custom := NewValidationError("custom failure", cfg, "Nested.Token")
	called := false
	cmd, err := NewCommand(cfg, "app", func(context.Context, *cli.Command) error { called = true; return nil }, WithCommandValidator(func(*cli.Command, any) error { return custom }))
	require.NoError(t, err)
	err = cmd.Run(t.Context(), []string{"app"})
	require.ErrorIs(t, err, custom)
	assert.False(t, called)
	for _, option := range []CommandOption{WithDisableValidation(), WithDisableCommandValidation()} {
		cmd, err = NewCommand(cfg, "app", nil, option)
		require.NoError(t, err)
		require.NoError(t, cmd.Run(t.Context(), []string{"app"}))
	}
	type Shared struct {
		Names []string `flag:"name" validate:"dive,required" global:"true" help:"names to select"`
	}
	embedded := &struct{ Shared }{Shared: Shared{Names: []string{"ok", ""}}}
	err = Validate(embedded)
	require.ErrorAs(t, err, &validation)
	assert.Equal(t, FieldInfo{Path: "Shared.Names[1]", Name: "--name", Help: "names to select"}, validation.Issues[0].Fields[0])
	custom = NewValidationError("conflict", embedded, "Names")
	assert.Equal(t, "--name", custom.Issues[0].Fields[0].Name)
	for _, invalidPath := range []string{"Nested.Token[0]", "Nested[0].Token", "Missing"} {
		custom = NewValidationError("invalid", cfg, invalidPath)
		assert.Equal(t, []FieldInfo{{Path: invalidPath}}, custom.Issues[0].Fields)
	}
	for _, invalidPath := range []string{"Names[-1]", "Names[", "Names[]", "Names[one]", "Names[0]extra", "Names[0][1]", "Names[+1]"} {
		custom = NewValidationError("invalid", embedded, invalidPath)
		assert.Equal(t, []FieldInfo{{Path: invalidPath}}, custom.Issues[0].Fields)
	}
	custom = NewValidationError("invalid", embedded, "Names[0]")
	assert.Equal(t, FieldInfo{Path: "Names[0]", Name: "--name", Help: "names to select"}, custom.Issues[0].Fields[0])
	for _, secret := range []string{"false", "true"} {
		t.Setenv("COUNTS", "1,private-invalid-value")
		typeOf := reflect.StructOf([]reflect.StructField{{Name: "Counts", Type: reflect.TypeFor[[]int](), Tag: reflect.StructTag(`env:"COUNTS" secret:"` + secret + `"`)}})
		err = LoadArgs(reflect.New(typeOf).Interface(), "app", []string{"app"})
		require.ErrorAs(t, err, &validation)
		assert.Equal(t, "--counts", validation.Issues[0].Fields[0].Name)
		if secret == "true" {
			assert.NotContains(t, err.Error(), "private-invalid-value")
		} else {
			assert.Contains(t, err.Error(), "element 1")
		}
	}
}

func Test_LoadArgsPrecedence(t *testing.T) {
	type config struct {
		Value  string `default:"value-from-default-tag"`
		Nested struct {
			Value string `default:"nested-value-from-default-tag"`
		}
		Doubly struct {
			Nested struct {
				Value string `default:"double-nested-value-from-default-tag"`
			}
		}
		Duration time.Duration `default:"10s"`
	}

	type args struct {
		cliArgs []string
		envVars map[string]string
		toml    string
	}

	tests := []struct {
		name            string
		args            *args
		wantValue       string
		wantNestedValue string
		wantDuration    time.Duration
	}{
		{
			name: "flags take precedence over everything else",
			args: &args{
				cliArgs: []string{"my-program", "--value", "value-from-cli", "--nested-value", "nested-value-from-cli", "--duration", "2s"},
				envVars: map[string]string{"VALUE": "value-from-env", "NESTED_VALUE": "nested-value-from-env", "DURATION": "1h1m1s"},
				toml: strings.TrimSpace(`
value = "value-from-toml"
nested.value = "nested-value-from-toml"
duration = "1m5s"
`),
			},
			wantValue:       "value-from-cli",
			wantNestedValue: "nested-value-from-cli",
			wantDuration:    2 * time.Second,
		},
		{
			name: "toml takes precedence over env vars and default values",
			args: &args{
				cliArgs: []string{"my-program"}, // no flags set
				envVars: map[string]string{"VALUE": "value-from-env", "NESTED_VALUE": "nested-value-from-env", "DURATION": "1h1m1s"},
				toml: strings.TrimSpace(`
value = "value-from-toml"
nested.value = "nested-value-from-toml"
duration = "1m5s"
`),
			},
			wantValue:       "value-from-toml",
			wantNestedValue: "nested-value-from-toml",
			wantDuration:    65 * time.Second,
		},
		{
			name: "env vars take precedence over default values",
			args: &args{
				cliArgs: []string{"my-program"}, // no flags set
				envVars: map[string]string{"VALUE": "value-from-env", "NESTED_VALUE": "nested-value-from-env", "DURATION": "1h1m1s"},
			},
			wantValue:       "value-from-env",
			wantNestedValue: "nested-value-from-env",
			wantDuration:    1*time.Hour + 1*time.Minute + 1*time.Second,
		},
		{
			name: "default values are used if nothing else is set",
			args: &args{
				cliArgs: []string{"my-program"}, // no flags set
				envVars: map[string]string{},
			},
			wantValue:       "value-from-default-tag",
			wantNestedValue: "nested-value-from-default-tag",
			wantDuration:    10 * time.Second,
		},
	}

	_ = tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &config{}

			// write our test toml file to a temporary file
			configPath := path.Join(t.TempDir(), "test-config.toml")
			require.NoError(t, os.WriteFile(configPath, []byte(tt.args.toml), 0o600))

			cliArgs := slices.Clone(tt.args.cliArgs)
			cliArgs = append(cliArgs, "--load-config", configPath)
			SetArgsForTest(t, cliArgs) // set cli args, and clean up after the test

			for key, value := range tt.args.envVars {
				t.Setenv(key, value) // set env vars, and clean up after the test
			}

			err := LoadArgs(config, "my-program", os.Args, WithDefaultLoadConfigFlag())
			require.NoError(t, err)

			assert.Equal(t, tt.wantValue, config.Value)
			assert.Equal(t, tt.wantNestedValue, config.Nested.Value)
			assert.Equal(t, tt.wantDuration, config.Duration)
		})
	}
}

func TestLoadConfigDiscovery(t *testing.T) {
	type config struct {
		Count  int    `env:"DISCOVERY_COUNT"`
		Counts []int  `env:"DISCOVERY_COUNTS"`
		Target string `arg:"0"                env:"-" nargs:"1"`
	}
	file := path.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(file, []byte("count = 17\ncounts = [3,7]\ntarget = 'toml'\n"), 0o600))
	for _, env := range []string{"DISCOVERY_COUNT", "DISCOVERY_COUNTS"} {
		t.Run(env, func(t *testing.T) {
			t.Setenv(env, "invalid")
			var cfg config
			err := LoadArgs(&cfg, "app", []string{"app", "--load-config", file}, WithDefaultLoadConfigFlag(), WithValidator(func(cmd *cli.Command, _ any) error {
				assert.Equal(t, []string{file}, cmd.StringSlice("load-config"))
				assert.Equal(t, 1, cmd.Count("load-config"))
				return nil
			}))
			require.NoError(t, err)
			assert.Equal(t, config{Count: 17, Counts: []int{3, 7}, Target: "toml"}, cfg)
			require.Error(t, LoadArgs(&cfg, "app", []string{"app", "target"}, WithDefaultLoadConfigFlag()))
			err = LoadArgs(&cfg, "app", []string{"app", "--load-config", file, "--count", "23", "--counts", "11,13", "cli"}, WithDefaultLoadConfigFlag())
			require.NoError(t, err)
			assert.Equal(t, config{Count: 23, Counts: []int{11, 13}, Target: "cli"}, cfg)
		})
	}
	var cfg config
	err := LoadArgs(&cfg, "app", []string{"app", "--help"}, WithDefaultLoadConfigFlag())
	var help *helpRequestedError
	require.ErrorAs(t, err, &help)
	assert.Contains(t, help.Error(), "[arguments...]")
	assert.Contains(t, help.Error(), "DISCOVERY_COUNT")
	assert.Contains(t, help.Error(), "--load-config")
	assert.Empty(t, cfg.Target)
}

func Test_LoadArgsMultipleTomlFilesPrecedence(t *testing.T) {
	type config struct {
		Value  string
		Second string
		Nested struct {
			Value  string
			Second string
		}
	}

	firstConfig := strings.TrimSpace(`
value = "first_config"
nested.value = "first_nested_config"
`)
	secondConfig := strings.TrimSpace(`
value = "second_config"
second= "second_config"

[nested]
value = "second_nested_config"
second = "second_nested_config"
`)

	firstConfigPath := path.Join(t.TempDir(), "first-config.toml")
	require.NoError(t, os.WriteFile(firstConfigPath, []byte(firstConfig), 0o600))

	secondConfigPath := path.Join(t.TempDir(), "second-config.toml")
	require.NoError(t, os.WriteFile(secondConfigPath, []byte(secondConfig), 0o600))

	SetArgsForTest(t, []string{"my-program", "--load-config", firstConfigPath + "," + secondConfigPath})

	cfg := &config{}
	err := LoadArgs(cfg, "my-program", os.Args, WithDefaultLoadConfigFlag())
	require.NoError(t, err)

	assert.Equal(t, "first_config", cfg.Value)
	assert.Equal(t, "first_nested_config", cfg.Nested.Value)
	assert.Equal(t, "second_config", cfg.Second)
	assert.Equal(t, "second_nested_config", cfg.Nested.Second)
}

func Test_LoadArgsExtraFlags(t *testing.T) {
	tests := []struct {
		name     string
		cfg      any
		loadOpts []Option
	}{
		{
			name: "plain struct config",
			cfg: &struct {
				SomeString string
				SomeInt    int
			}{},
			loadOpts: []Option{},
		},
		{
			name: "with load config flag",
			cfg: &struct {
				SomeString string
				SomeInt    int
			}{},
			loadOpts: []Option{WithDefaultLoadConfigFlag()},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetArgsForTest(t, []string{"my-program", "--some-string", "hello", "--some-int", "42", "--unknown-flag", "value"})

			err := LoadArgs(tt.cfg, "my-program", os.Args, tt.loadOpts...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "flag provided but not defined: -unknown-flag")
			assert.Contains(t, err.Error(), "USAGE:")
		})
	}
}

func Test_PrintCorrectUsage(t *testing.T) {
	type config struct {
		Value            string
		DocumentedValue  string `help:"Description of the documented value"`
		ValueWithDefault string `default:"default"                          help:"A documented value that has a default"`
	}

	SetArgsForTest(t, []string{"my-program", "--unknown-value", "to_trigger_usage"})

	err := LoadArgs(&config{}, "my-program", os.Args)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "--documented-value string    Description of the documented value [$DOCUMENTED_VALUE]")
	assert.Contains(t, err.Error(), "--value-with-default string  A documented value that has a default (default: default) [$VALUE_WITH_DEFAULT]")
}

func Test_LoadArgsDuplicates(t *testing.T) {
	tests := []struct {
		name      string
		cfg       any
		wantError string
	}{
		{
			name: "duplicate flag names disallowed",
			cfg: &struct {
				Value  string `flag:"value"`
				Value2 string `flag:"value"`
			}{},
			wantError: "duplicate flag: --value",
		},
		{
			name: "duplicate flag names are allowed in nested structs",
			cfg: &struct {
				Obj struct {
					Value string `flag:"value"`
				}
				Other struct {
					Value string `flag:"value"`
				}
			}{},
			wantError: "", // in nested structs it is allowed
		},
		{
			name: "duplicate env names allowed",
			cfg: &struct {
				Value  string `env:"VALUE"`
				Value2 string `env:"VALUE"`
			}{},
			wantError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetArgsForTest(t, []string{"my-program"}) // no args set

			err := LoadArgs(tt.cfg, "my-program", os.Args)
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_LoadArgs(t *testing.T) {
	type config struct {
		Name string `validate:"required"`
	}

	SetArgsForTest(t, []string{"my-program"})

	cfg := &config{}
	err := LoadArgs(cfg, "my-program", []string{"my-program", "--name", "Tilebox"})
	require.NoError(t, err)
	assert.Equal(t, "Tilebox", cfg.Name)
}

func Test_LoadArgsArgumentBinding(t *testing.T) {
	type config struct {
		SomeFlag  string `flag:"some-flag"`
		SecondArg string `arg:"1"`
		OtherFlag bool   `flag:"other-flag"`
		FirstArg  int    `arg:"0"`
	}

	cfg := &config{}
	err := LoadArgs(cfg, "my-program", []string{"my-program", "--some-flag", "from-flag", "42", "from-arg", "--other-flag"})
	require.NoError(t, err)

	assert.Equal(t, "from-flag", cfg.SomeFlag)
	assert.True(t, cfg.OtherFlag)
	assert.Equal(t, 42, cfg.FirstArg)
	assert.Equal(t, "from-arg", cfg.SecondArg)
}

func Test_LoadArgsArgumentFallbacks(t *testing.T) {
	type config struct {
		Name  string `arg:"0" env:"NAME"  default:"World" toml:"name"`
		Count int    `arg:"1" env:"COUNT" default:"1"     toml:"count"`
	}

	tests := []struct {
		name      string
		cliArgs   []string
		envVars   map[string]string
		toml      string
		wantName  string
		wantCount int
	}{
		{
			name:      "positional args take precedence",
			cliArgs:   []string{"my-program", "Tilebox", "3"},
			envVars:   map[string]string{"NAME": "from-env", "COUNT": "2"},
			toml:      "name = \"from-toml\"\ncount = 4",
			wantName:  "Tilebox",
			wantCount: 3,
		},
		{
			name:      "later args fall back independently",
			cliArgs:   []string{"my-program", "Tilebox"},
			envVars:   map[string]string{"COUNT": "2"},
			wantName:  "Tilebox",
			wantCount: 2,
		},
		{
			name:      "toml takes precedence over env vars and defaults",
			cliArgs:   []string{"my-program"},
			envVars:   map[string]string{"NAME": "from-env", "COUNT": "2"},
			toml:      "name = \"from-toml\"\ncount = 4",
			wantName:  "from-toml",
			wantCount: 4,
		},
		{
			name:      "env vars take precedence over defaults",
			cliArgs:   []string{"my-program"},
			envVars:   map[string]string{"NAME": "from-env", "COUNT": "2"},
			wantName:  "from-env",
			wantCount: 2,
		},
		{
			name:      "defaults are used if nothing else is set",
			cliArgs:   []string{"my-program"},
			wantName:  "World",
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cliArgs := slices.Clone(tt.cliArgs)
			if tt.toml != "" {
				configPath := path.Join(t.TempDir(), "test-config.toml")
				require.NoError(t, os.WriteFile(configPath, []byte(tt.toml), 0o600))
				cliArgs = append(cliArgs, "--load-config", configPath)
			}

			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			cfg := &config{}
			err := LoadArgs(cfg, "my-program", cliArgs, WithDefaultLoadConfigFlag())
			require.NoError(t, err)

			assert.Equal(t, tt.wantName, cfg.Name)
			assert.Equal(t, tt.wantCount, cfg.Count)
		})
	}
}

func Test_LoadArgsArgumentEmptySources(t *testing.T) {
	type config struct {
		Name    string        `arg:"0" env:"TEST_ARG_NAME"    default:"default-name"`
		Count   int           `arg:"1" env:"TEST_ARG_COUNT"   default:"3"`
		Limit   uint          `arg:"2" env:"TEST_ARG_LIMIT"   default:"5"`
		Ratio   float64       `arg:"3" env:"TEST_ARG_RATIO"   default:"1.5"`
		Timeout time.Duration `arg:"4" env:"TEST_ARG_TIMEOUT" default:"4s"`
		Enabled bool          `arg:"5" env:"TEST_ARG_ENABLED" default:"true"`
		Values  []string      `arg:"6" env:"TEST_ARG_VALUES"  default:"one,two"`
	}

	tests := []struct {
		name      string
		toml      string
		env       map[string]string
		args      []string
		want      config
		wantError string
	}{
		{
			name: "empty env uses numeric defaults but clears strings slices and bools",
			want: config{Count: 3, Limit: 5, Ratio: 1.5, Timeout: 4 * time.Second},
		},
		{
			name: "empty toml falls through to env only for numeric types",
			toml: "name = \"\"\ncount = \"\"\nlimit = \"\"\nratio = \"\"\ntimeout = \"\"\nenabled = \"\"\nvalues = []",
			env: map[string]string{
				"TEST_ARG_NAME": "env-name", "TEST_ARG_COUNT": "7", "TEST_ARG_LIMIT": "11",
				"TEST_ARG_RATIO": "2.5", "TEST_ARG_TIMEOUT": "9s", "TEST_ARG_ENABLED": "true", "TEST_ARG_VALUES": "env-value",
			},
			want: config{Count: 7, Limit: 11, Ratio: 2.5, Timeout: 9 * time.Second},
		},
		{
			name: "empty toml and env fall through to numeric defaults",
			toml: "count = \"\"\nlimit = \"\"\nratio = \"\"\ntimeout = \"\"",
			want: config{Count: 3, Limit: 5, Ratio: 1.5, Timeout: 4 * time.Second},
		},
		{
			name: "explicit empty string does not fall back",
			args: []string{""},
			env:  map[string]string{"TEST_ARG_NAME": "env-name"},
			want: config{Count: 3, Limit: 5, Ratio: 1.5, Timeout: 4 * time.Second},
		},
		{
			name:      "explicit empty number is an error",
			args:      []string{"name", ""},
			env:       map[string]string{"TEST_ARG_COUNT": "7"},
			wantError: "invalid value \"\" for argument count",
		},
		{
			name:      "invalid nonempty toml does not fall back",
			toml:      "count = \"invalid\"",
			env:       map[string]string{"TEST_ARG_COUNT": "7"},
			wantError: "failed to parse source value invalid for argument count",
		},
		{
			name:      "whitespace env is not treated as empty",
			env:       map[string]string{"TEST_ARG_COUNT": " "},
			wantError: "failed to parse source value   for argument count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, key := range []string{"TEST_ARG_NAME", "TEST_ARG_COUNT", "TEST_ARG_LIMIT", "TEST_ARG_RATIO", "TEST_ARG_TIMEOUT", "TEST_ARG_ENABLED", "TEST_ARG_VALUES"} {
				t.Setenv(key, tt.env[key])
			}
			args := []string{"app"}
			if tt.toml != "" {
				configPath := path.Join(t.TempDir(), "config.toml")
				require.NoError(t, os.WriteFile(configPath, []byte(tt.toml), 0o600))
				args = append(args, "--load-config", configPath)
			}
			args = append(args, tt.args...)
			cfg := &config{}
			err := LoadArgs(cfg, "app", args, WithDefaultLoadConfigFlag())
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, *cfg)
		})
	}
}

func Test_LoadArgsArgumentValidation(t *testing.T) {
	type config struct {
		Name string `arg:"0" validate:"required"`
	}

	cfg := &config{}
	err := LoadArgs(cfg, "my-program", []string{"my-program"})
	require.Error(t, err)
	assert.EqualError(t, err, "<name> is required")
}

func Test_LoadArgsArgumentErrors(t *testing.T) {
	tests := []struct {
		name      string
		cfg       any
		cliArgs   []string
		wantError string
	}{
		{
			name: "duplicate indexes disallowed",
			cfg: &struct {
				First  string `arg:"0"`
				Second string `arg:"0"`
			}{},
			cliArgs:   []string{"my-program"},
			wantError: "duplicate argument index: 0",
		},
		{
			name: "gaps disallowed",
			cfg: &struct {
				First string `arg:"0"`
				Third string `arg:"2"`
			}{},
			cliArgs:   []string{"my-program"},
			wantError: "missing argument binding for index: 1",
		},
		{
			name: "invalid index disallowed",
			cfg: &struct {
				Name string `arg:"not-a-number"`
			}{},
			cliArgs:   []string{"my-program"},
			wantError: "invalid argument index \"not-a-number\" for field Name",
		},
		{
			name: "argument and flag binding disallowed",
			cfg: &struct {
				Name string `flag:"name" arg:"0"`
			}{},
			cliArgs:   []string{"my-program"},
			wantError: "field Name cannot be bound as both argument and flag",
		},
		{
			name: "invalid argument value returns parse error",
			cfg: &struct {
				Count int `arg:"0"`
			}{},
			cliArgs:   []string{"my-program", "not-a-number"},
			wantError: "invalid value \"not-a-number\" for argument count",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := LoadArgs(tt.cfg, "my-program", tt.cliArgs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantError)
		})
	}
}

func Test_LoadArgsUsesCustomValidator(t *testing.T) {
	type config struct {
		Name string `validate:"required"`
	}

	cfg := &config{}
	customErr := errors.New("custom validation failed")

	err := LoadArgs(cfg, "my-program", []string{"my-program", "--name", "Tilebox"}, WithValidator(func(cmd *cli.Command, configPointer any) error {
		assert.Equal(t, "my-program", cmd.Name)
		assert.Same(t, cfg, configPointer)
		assert.Equal(t, "Tilebox", cfg.Name)
		return customErr
	}))
	require.ErrorIs(t, err, customErr)
}

func Test_LoadArgsCustomValidatorReplacesDefaultValidator(t *testing.T) {
	type config struct {
		Name string `validate:"required"`
	}

	cfg := &config{}
	err := LoadArgs(cfg, "my-program", []string{"my-program"}, WithValidator(func(cmd *cli.Command, configPointer any) error {
		assert.Equal(t, "my-program", cmd.Name)
		assert.Same(t, cfg, configPointer)
		return nil
	}))
	require.NoError(t, err)
}

func Test_NewCommandSubcommands(t *testing.T) {
	type greetConfig struct {
		Name string `default:"World"`
		Loud bool
	}
	type sumConfig struct {
		Left  int
		Right int
	}

	greetCfg := &greetConfig{}
	sumCfg := &sumConfig{}

	greetRan := false
	sumRan := false

	greetCmd, err := NewCommand(greetCfg, "greet", func(ctx context.Context, cmd *cli.Command) error {
		greetRan = true
		if greetCfg.Loud {
			greetCfg.Name = strings.ToUpper(greetCfg.Name)
		}
		return nil
	})
	require.NoError(t, err)

	sumCmd, err := NewCommand(sumCfg, "sum", func(ctx context.Context, cmd *cli.Command) error {
		sumRan = true
		return nil
	})
	require.NoError(t, err)

	root := &cli.Command{
		Name:     "app",
		Commands: []*cli.Command{greetCmd, sumCmd},
	}

	err = root.Run(context.Background(), []string{"app", "greet", "--name", "tilebox", "--loud"})
	require.NoError(t, err)

	assert.True(t, greetRan)
	assert.False(t, sumRan)
	assert.Equal(t, "TILEBOX", greetCfg.Name)
	assert.Equal(t, 0, sumCfg.Left)
	assert.Equal(t, 0, sumCfg.Right)
}

func Test_NewCommandBindsArguments(t *testing.T) {
	type config struct {
		Name string `arg:"0" help:"Name to greet"`
		Loud bool
	}

	cfg := &config{}
	actionRan := false

	cmd, err := NewCommand(cfg, "greet", func(ctx context.Context, cmd *cli.Command) error {
		actionRan = true
		return nil
	})
	require.NoError(t, err)

	err = cmd.Run(context.Background(), []string{"greet", "Tilebox", "--loud"})
	require.NoError(t, err)

	assert.True(t, actionRan)
	assert.Equal(t, "Tilebox", cfg.Name)
	assert.True(t, cfg.Loud)

	metadata, ok := cmd.Arguments[0].(ArgumentMetadata)
	require.True(t, ok)
	assert.Equal(t, "name", metadata.Name())
	assert.Equal(t, "string", metadata.TypeName())
	assert.Equal(t, "Name to greet", metadata.UsageText())
	assert.Equal(t, "name", cmd.Arguments[0].Usage())
}

func Test_BindCommandValidatesBeforeAction(t *testing.T) {
	type config struct {
		Name string `validate:"required"`
	}

	cfg := &config{}
	actionRan := false
	cmd := &cli.Command{
		Name: "greet",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			actionRan = true
			return nil
		},
	}

	err := BindCommand(cmd, cfg)
	require.NoError(t, err)

	err = cmd.Run(context.Background(), []string{"greet"})
	require.EqualError(t, err, "--name is required")
	assert.False(t, actionRan)
}

func Test_BindCommandUsesCustomValidator(t *testing.T) {
	type config struct {
		Name string `validate:"required"`
	}

	cfg := &config{}
	actionRan := false
	customErr := errors.New("custom command validation failed")
	cmd := &cli.Command{
		Name: "greet",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			actionRan = true
			return nil
		},
	}

	err := BindCommand(cmd, cfg, WithValidator(func(cmd *cli.Command, configPointer any) error {
		assert.Equal(t, "greet", cmd.Name)
		assert.Same(t, cfg, configPointer)
		assert.Equal(t, "Tilebox", cfg.Name)
		return customErr
	}))
	require.NoError(t, err)

	err = cmd.Run(context.Background(), []string{"greet", "--name", "Tilebox"})
	require.ErrorIs(t, err, customErr)
	assert.False(t, actionRan)
}

func TestMarshalPreservesNestedPointers(t *testing.T) {
	type nested struct{ Enabled bool }
	cfg := &struct {
		Absent  *nested
		Present *nested
	}{Present: &nested{}}
	asMap, err := MarshalAsMap(cfg)
	require.NoError(t, err)
	assert.Nil(t, cfg.Absent)
	assert.Equal(t, map[string]any{"present": map[string]any{"enabled": false}}, asMap)
	attr, err := MarshalAsSlogDict(cfg, "config")
	require.NoError(t, err)
	assert.Nil(t, cfg.Absent)
	assert.Equal(t, slog.Group("config", slog.Group("present", slog.Bool("enabled", false))), attr)
}

func Test_MarshalLoadedStringSlice(t *testing.T) {
	cfg := &struct {
		EnvironmentEncryption struct {
			KeyFiles []string
		}
	}{}
	require.NoError(t, LoadArgs(cfg, "test", []string{
		"test", "--environment-encryption-key-files=old=/tmp/old.key,new=/tmp/new.key",
	}))

	asMap, err := MarshalAsMap(cfg)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"environment-encryption": map[string]any{
			"key-files": []any{"old=/tmp/old.key", "new=/tmp/new.key"},
		},
	}, asMap)

	attr, err := MarshalAsSlogDict(cfg, "config")
	require.NoError(t, err)
	assert.Equal(t, slog.Group("config", slog.Group("environment-encryption",
		slog.Any("key-files", []any{"old=/tmp/old.key", "new=/tmp/new.key"}),
	)), attr)
}

func Test_MarshalStringSlices(t *testing.T) {
	for _, tt := range []struct {
		name       string
		values     []string
		wantValues []any
		wantLog    string
	}{
		{name: "nil"},
		{name: "empty", values: []string{}, wantValues: []any{}, wantLog: "[]"},
		{
			name:       "secret elements",
			values:     []string{"abc", "abcde", "long-secret-value", ""},
			wantValues: []any{"***", "ab***de", "lo***ue", "***"},
			wantLog:    "[*** ab***de lo***ue ***]",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &struct {
				Values []string `secret:"true"`
			}{Values: slices.Clone(tt.values)}
			wantMap := map[string]any{}
			var wantAttrs []slog.Attr
			if tt.values != nil {
				wantMap["values"] = tt.wantValues
				wantAttrs = append(wantAttrs, slog.Any("values", tt.wantValues))
			}

			asMap, err := MarshalAsMap(cfg)
			require.NoError(t, err)
			assert.Equal(t, wantMap, asMap)

			attr, err := MarshalAsSlogDict(cfg, "config")
			require.NoError(t, err)
			assert.Equal(t, "config", attr.Key)
			assert.ElementsMatch(t, wantAttrs, attr.Value.Group())
			if tt.values != nil {
				assert.Equal(t, tt.wantLog, attr.Value.Group()[0].Value.String())
			}
			assert.Equal(t, tt.values, cfg.Values, "marshaling must not modify the config")
		})
	}
}

func Test_MarshalPrimitiveSlices(t *testing.T) {
	for _, tt := range []struct {
		name       string
		config     any
		wantValues []any
		wantLog    string
	}{
		{"int", &struct{ Values []int }{[]int{-42, 0, 17}}, []any{int64(-42), int64(0), int64(17)}, "[-42 0 17]"},
		{"int8", &struct{ Values []int8 }{[]int8{-128, 127}}, []any{int64(-128), int64(127)}, "[-128 127]"},
		{"int16", &struct{ Values []int16 }{[]int16{-32768, 32767}}, []any{int64(-32768), int64(32767)}, "[-32768 32767]"},
		{"int32", &struct{ Values []int32 }{[]int32{-2147483648, 2147483647}}, []any{int64(-2147483648), int64(2147483647)}, "[-2147483648 2147483647]"},
		{"int64", &struct{ Values []int64 }{[]int64{-1 << 63, 1<<63 - 1}}, []any{int64(-1 << 63), int64(1<<63 - 1)}, "[-9223372036854775808 9223372036854775807]"},
		{"uint", &struct{ Values []uint }{[]uint{0, 42}}, []any{uint64(0), uint64(42)}, "[0 42]"},
		{"uint8", &struct{ Values []uint8 }{[]uint8{0, 255}}, []any{uint64(0), uint64(255)}, "[0 255]"},
		{"uint16", &struct{ Values []uint16 }{[]uint16{0, 65535}}, []any{uint64(0), uint64(65535)}, "[0 65535]"},
		{"uint32", &struct{ Values []uint32 }{[]uint32{0, 4294967295}}, []any{uint64(0), uint64(4294967295)}, "[0 4294967295]"},
		{"uint64", &struct{ Values []uint64 }{[]uint64{0, 1<<64 - 1}}, []any{uint64(0), uint64(1<<64 - 1)}, "[0 18446744073709551615]"},
		{"float32", &struct{ Values []float32 }{[]float32{-1.5, 0, 2.25}}, []any{float64(-1.5), float64(0), float64(2.25)}, "[-1.5 0 2.25]"},
		{"float64", &struct{ Values []float64 }{[]float64{-3.125, 0, 8.5}}, []any{-3.125, float64(0), 8.5}, "[-3.125 0 8.5]"},
		{"bool", &struct{ Values []bool }{[]bool{false, true}}, []any{false, true}, "[false true]"},
		{"duration", &struct{ Values []time.Duration }{[]time.Duration{-time.Second, 0, 1500 * time.Millisecond}}, []any{"-1s", "0s", "1.5s"}, "[-1s 0s 1.5s]"},
		{"nested", &struct{ Values [][]int }{[][]int{{-3, 0}, {7}}}, []any{[]any{int64(-3), int64(0)}, []any{int64(7)}}, "[[-3 0] [7]]"},
		{
			"nested secrets",
			&struct {
				Values [][]string `secret:"true"`
			}{[][]string{{"abc", "long-secret-value"}, {"abcde"}}},
			[]any{[]any{"***", "lo***ue"}, []any{"ab***de"}},
			"[[*** lo***ue] [ab***de]]",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			asMap, err := MarshalAsMap(tt.config)
			require.NoError(t, err)
			assert.Equal(t, map[string]any{"values": tt.wantValues}, asMap)

			attr, err := MarshalAsSlogDict(tt.config, "config")
			require.NoError(t, err)
			assert.Equal(t, slog.Group("config", slog.Any("values", tt.wantValues)), attr)
			assert.Equal(t, tt.wantLog, attr.Value.Group()[0].Value.String())
		})
	}

	t.Run("unsupported elements", func(t *testing.T) {
		cfg := &struct{ Values []map[string]int }{Values: []map[string]int{{"key": 1}}}
		_, err := MarshalAsMap(cfg)
		require.EqualError(t, err, "slice element 0: unknown field type map")
		_, err = MarshalAsSlogDict(cfg, "config")
		require.EqualError(t, err, "slice element 0: unknown field type map")
	})
}

func Test_MarshalAsSlogDictJSON(t *testing.T) {
	type nestedConfig struct {
		Count   int
		Limit   uint64
		Ratio   float64
		Enabled bool
		Timeout time.Duration
		Token   string   `secret:"true"`
		Tokens  []string `secret:"true"`
		Matrix  [][]int
		Empty   []bool
		Absent  []string
	}
	cfg := &struct {
		Name   string
		Active bool
		Nested nestedConfig
	}{
		Name:   "worker",
		Active: true,
		Nested: nestedConfig{
			Count: -7, Limit: 42, Ratio: 1.25,
			Timeout: 1500 * time.Millisecond,
			Token:   "secret-token",
			Tokens:  []string{"abc", "long-secret-value"},
			Matrix:  [][]int{{-3, 0}, {7}},
			Empty:   []bool{},
		},
	}
	attr, err := MarshalAsSlogDict(cfg, "config")
	require.NoError(t, err)

	var output strings.Builder
	handler := slog.NewJSONHandler(&output, nil)
	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "loaded", 0)
	record.AddAttrs(attr)
	require.NoError(t, handler.Handle(t.Context(), record))
	assert.JSONEq(t, `{
		"level": "INFO", "msg": "loaded",
		"config": {
			"name": "worker", "active": true,
			"nested": {
				"count": -7, "limit": 42, "ratio": 1.25, "enabled": false,
				"timeout": "1.5s", "token": "se***en",
				"tokens": ["***", "lo***ue"], "matrix": [[-3, 0], [7]], "empty": []
			}
		}
	}`, output.String())
}

func SetArgsForTest(t *testing.T, args []string) {
	oldArgs := os.Args

	t.Cleanup(func() {
		os.Args = oldArgs
	})

	os.Args = args
}

func TestValidationMessages(t *testing.T) {
	for _, tc := range []struct {
		name, rule, want string
		invalid, valid   any
	}{
		{"required", "required", "--value is required", "", "present"},
		{"choices", "oneof='red green' blue", "--value must be one of: 'red green' blue", "private-input", "red green"},
		{"case insensitive choices", "oneofci=red blue", "--value must be one of: red blue", "private-input", "RED"},
		{"optional string", "omitempty,min=1", "--value must contain at least 1 character", new(""), (*string)(nil)},
		{"unicode length", "min=2", "--value must contain at least 2 characters", "é", "éø"},
		{"maximum length", "max=1", "--value must contain at most 1 character", "ab", "é"},
		{"exact length", "len=2", "--value must contain exactly 2 characters", "a", "ab"},
		{"slice minimum", "min=1", "--value must contain at least 1 item", []string{}, []string{"a"}},
		{"map maximum", "max=1", "--value must contain at most 1 item", map[string]int{"a": 1, "b": 2}, map[string]int{"a": 1}},
		{"slice exact length", "len=2", "--value must contain exactly 2 items", []int{1}, []int{1, 2}},
		{"numeric minimum", "min=2", "--value must be at least 2", 1, 2},
		{"numeric maximum", "max=2", "--value must be at most 2", 3, 2},
		{"numeric exact", "len=2", "--value must be exactly 2", 3, 2},
		{"inclusive lower bound", "gte=0", "--value must be at least 0", new(-1), new(0)},
		{"inclusive upper bound", "lte=100", "--value must be at most 100", uint(101), uint(100)},
		{"exclusive lower bound", "gt=0", "--value must be greater than 0", 0, 1},
		{"exclusive upper bound", "lt=1.5", "--value must be less than 1.5", 1.5, 1.4},
		{"string lower bound", "gt=1", "--value must contain more than 1 character", "a", "ab"},
		{"slice upper bound", "lt=2", "--value must contain fewer than 2 items", []int{1, 2}, []int{1}},
		{"string inequality", "ne=reserved", `--value must not equal "reserved"`, "reserved", "allowed"},
		{"number equality", "eq=2", "--value must equal 2", 3, 2},
		{"slice equality", "eq=1", "--value must contain exactly 1 item", []int{1, 2}, []int{1}},
		{"slice inequality", "ne=2", "--value must not contain exactly 2 items", []int{1, 2}, []int{1}},
		{"uuid", "uuid", "--value must be a valid UUID", "private-input", "019e4f3c-4646-7312-b8fe-2e7fa83c1546"},
		{"url", "url", "--value must be a valid URL", "private-input", "https://example.com"},
		{"email", "email", "--value must be a valid email address", "private-input", "user@example.com"},
		{"letters", "alpha", "--value must contain only letters", "abc1", "abc"},
		{"alphanumeric", "alphanum", "--value must contain only letters and digits", "abc-1", "abc1"},
		{"contains", "contains=prefix", `--value must contain "prefix"`, "private-input", "prefix-value"},
		{"excludes", "excludes=/", `--value must not contain "/"`, "a/b", "ab"},
		{"prefix", "startswith=prefix", `--value must start with "prefix"`, "private-input", "prefix-value"},
		{"suffix", "endswith=.json", `--value must end with ".json"`, "private-input", "config.json"},
		{"fallback", "hexcolor", "--value does not satisfy the hexcolor constraint", "private-input", "#ffffff"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			typeOf := reflect.StructOf([]reflect.StructField{{
				Name: "Value", Type: reflect.TypeOf(tc.invalid),
				Tag: reflect.StructTag(fmt.Sprintf(`flag:"value" secret:"true" validate:%q help:"input value"`, tc.rule)),
			}})
			config := reflect.New(typeOf)
			config.Elem().Field(0).Set(reflect.ValueOf(tc.invalid))
			err := Validate(config.Interface())
			require.EqualError(t, err, tc.want)
			var validation *ValidationError
			require.ErrorAs(t, err, &validation)
			require.Len(t, validation.Issues, 1)
			assert.Equal(t, []FieldInfo{{Path: "Value", Name: "--value", Help: "input value"}}, validation.Issues[0].Fields)
			assert.NotContains(t, err.Error(), "private-input")
			config.Elem().Field(0).Set(reflect.ValueOf(tc.valid))
			assert.NoError(t, Validate(config.Interface()))
		})
	}

	t.Run("unbound field", func(t *testing.T) {
		config := &struct {
			Name string `flag:"-" validate:"required"`
		}{}
		assert.EqualError(t, Validate(config), "Name is required")
	})
}

func Test_validate(t *testing.T) {
	type config struct {
		RequiredValue      string `validate:"required"`
		Alphanumeric       string `validate:"alphanum"`
		Contains           string `validate:"contains=MustContain"`
		NumberBetween0to10 int    `validate:"gte=0,lte=10"`
	}

	type args struct {
		config any
	}

	tests := []struct {
		name    string
		args    args
		wantErr string
	}{
		{
			name: "valid config",
			args: args{
				config: &config{
					RequiredValue:      "value",
					Alphanumeric:       "alphanumeric",
					Contains:           "some config value that must have MustContain in it",
					NumberBetween0to10: 5,
				},
			},
			wantErr: "",
		},
		{
			name: "missing required value",
			args: args{
				config: &config{
					Alphanumeric:       "alphanumeric",
					Contains:           "some config value that must have MustContain in it",
					NumberBetween0to10: 5,
				},
			},
			wantErr: "--required-value is required",
		},
		{
			name: "multiple errors",
			args: args{
				config: &config{
					NumberBetween0to10: -1,
				},
			},
			wantErr: strings.TrimSpace(`
--required-value is required
--alphanumeric must contain only letters and digits
--contains must contain "MustContain"
--number-between-0-to-10 must be at least 0
`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(nil, tt.args.config)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
