package structconf

import (
	"context"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

func Test_loadConfigFullyTagged(t *testing.T) {
	type config struct {
		Value  string `flag:"value" env:"VALUE" default:"value-from-default-tag" toml:"value"`
		Nested struct {
			Value string `flag:"value" env:"VALUE" default:"nested-value-from-default-tag" toml:"value"`
		} `flag:"nested" toml:"nested" env:"NESTED"`
		Doubly struct {
			Nested struct {
				Value string `flag:"value" env:"VALUE" default:"double-nested-value-from-default-tag" toml:"value"`
			} `flag:"nested" env:"NESTED" toml:"nested"`
		} `flag:"doubly" toml:"doubly" env:"DOUBLY"`
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

			err := loadConfigWithArgs(config, "my-program", os.Args, WithDefaultLoadConfigFlag())
			require.NoError(t, err)

			assert.Equal(t, tt.wantValue, config.Value)
			assert.Equal(t, tt.wantNestedValue, config.Nested.Value)
			assert.Equal(t, tt.wantDoublyNestedValue, config.Doubly.Nested.Value)
			assert.Equal(t, tt.wantDuration, config.Duration)
		})
	}
}

func Test_loadConfigDefaultTags(t *testing.T) {
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

			err := loadConfigWithArgs(config, "my-program", os.Args, WithDefaultLoadConfigFlag())
			require.NoError(t, err)

			assert.Equal(t, tt.wantValue, config.Value)
			assert.Equal(t, tt.wantNestedValue, config.Nested.Value)
			assert.Equal(t, tt.wantDoublyNestedValue, config.Doubly.Nested.Value)
			assert.Equal(t, tt.wantDuration, config.Duration)
		})
	}
}

func Test_loadConfigPrecedence(t *testing.T) {
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

			err := loadConfigWithArgs(config, "my-program", os.Args, WithDefaultLoadConfigFlag())
			require.NoError(t, err)

			assert.Equal(t, tt.wantValue, config.Value)
			assert.Equal(t, tt.wantNestedValue, config.Nested.Value)
			assert.Equal(t, tt.wantDuration, config.Duration)
		})
	}
}

func Test_loadConfigMultipleTomlFilesPrecedence(t *testing.T) {
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
	err := loadConfigWithArgs(cfg, "my-program", os.Args, WithDefaultLoadConfigFlag())
	require.NoError(t, err)

	assert.Equal(t, "first_config", cfg.Value)
	assert.Equal(t, "first_nested_config", cfg.Nested.Value)
	assert.Equal(t, "second_config", cfg.Second)
	assert.Equal(t, "second_nested_config", cfg.Nested.Second)
}

func Test_loadConfigExtraFlags(t *testing.T) {
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

			err := loadConfigWithArgs(tt.cfg, "my-program", os.Args, tt.loadOpts...)
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

	err := loadConfigWithArgs(&config{}, "my-program", os.Args)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "--documented-value string    Description of the documented value [$DOCUMENTED_VALUE]")
	assert.Contains(t, err.Error(), "--value-with-default string  A documented value that has a default (default: default) [$VALUE_WITH_DEFAULT]")
}

func Test_loadConfigDuplicates(t *testing.T) {
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

			err := loadConfigWithArgs(tt.cfg, "my-program", os.Args)
			if tt.wantError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func Test_LoadAndValidateArgs(t *testing.T) {
	type config struct {
		Name string `validate:"required"`
	}

	SetArgsForTest(t, []string{"my-program"})

	cfg := &config{}
	err := LoadAndValidateArgs(cfg, "my-program", []string{"my-program", "--name", "Tilebox"})
	require.NoError(t, err)
	assert.Equal(t, "Tilebox", cfg.Name)
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

func Test_BindCommandRejectsLoadConfigFlag(t *testing.T) {
	type config struct {
		Name string
	}

	cmd := &cli.Command{Name: "greet"}
	err := BindCommand(cmd, &config{}, WithDefaultLoadConfigFlag())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "WithLoadConfigFlag is not supported")
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
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Missing required configuration")
	assert.False(t, actionRan)
}

func SetArgsForTest(t *testing.T, args []string) {
	oldArgs := os.Args

	t.Cleanup(func() {
		os.Args = oldArgs
	})

	os.Args = args
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
			wantErr: "Missing required configuration: config.RequiredValue",
		},
		{
			name: "multiple errors",
			args: args{
				config: &config{
					NumberBetween0to10: -1,
				},
			},
			wantErr: strings.TrimSpace(`
Missing required configuration: config.RequiredValue
Configuration error: Alphanumeric - alphanum
Configuration error: Contains - contains
Configuration error: NumberBetween0to10 - gte
`),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate(tt.args.config)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
