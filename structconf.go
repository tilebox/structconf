package structconf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
)

type options struct {
	version               string
	description           string
	longDescription       string
	enableShellCompletion bool
	loadConfigFlagName    string
}

type Option func(opts *options)

func WithVersion(version string) Option {
	return func(opts *options) {
		opts.version = version
	}
}

func WithDescription(description string) Option {
	return func(opts *options) {
		opts.description = description
	}
}

func WithLongDescription(usage string) Option {
	return func(opts *options) {
		opts.longDescription = usage
	}
}

func WithShellCompletions() Option {
	return func(opts *options) {
		opts.enableShellCompletion = true
	}
}

func WithDefaultLoadConfigFlag() Option {
	return WithLoadConfigFlag("load-config")
}

func WithLoadConfigFlag(flagName string) Option {
	return func(opts *options) {
		opts.loadConfigFlagName = flagName
	}
}

// MustLoadAndValidate is like LoadAndValidate, but if it fails, it prints the error to stderr and exits
// with a non-zero exit code.
func MustLoadAndValidate(configPointer any, programName string, opts ...Option) {
	err := LoadAndValidate(configPointer, programName, opts...)
	if err != nil {
		helpRequested := &helpRequestedError{}
		if errors.As(err, &helpRequested) {
			fmt.Print(helpRequested.helpText) //nolint:forbidigo
			os.Exit(0)                        // no error, since we requested help
		}

		_, err = fmt.Fprintln(os.Stderr, err.Error())
		if err != nil {
			fmt.Print(err.Error()) //nolint:forbidigo    // we couldn't print to stderr, so let's print to stdout instead
		}
		os.Exit(1)
	}
}

// LoadAndValidate loads the given config struct and validates it.
//
// It loads the config from the following sources in the given order:
// 1. command line flags
// 2. config files (if the config struct satisfies the loadConfigFromTOMLFiles interface by embedding LoadTOMLConfig)
// 3. environment variables
// 4. default values defined in the field tags
//
// It then validates the loaded config, using the validate tag in config fields - if it fails, it returns an error.
// The returned error is suitable to be printed to the user.
func LoadAndValidate(configPointer any, programName string, opts ...Option) error {
	err := loadConfig(configPointer, programName, opts...)
	if err != nil {
		return err
	}

	return validate(configPointer)
}

type helpRequestedError struct {
	helpText string
}

func (e *helpRequestedError) Error() string {
	return e.helpText
}

func loadConfig(configPointer any, programName string, opts ...Option) error {
	cfg := &options{}
	for _, opt := range opts {
		opt(cfg)
	}

	tomlSources := make([]cli.MapSource, 0)
	var loadConfigFlag cli.Flag
	if cfg.loadConfigFlagName != "" {
		loadConfigFlag = &cli.StringSliceFlag{
			Name:  cfg.loadConfigFlagName,
			Usage: "Load configuration from TOML files",
		}

		config, err := NewStructConfigurator(configPointer, nil)
		if err != nil {
			return err
		}
		flags := config.Flags()
		flags = append(flags, loadConfigFlag)
		if duplicate := firstDuplicateFlagName(flags); duplicate != "" {
			return fmt.Errorf("got duplicate flag name: %s", duplicate)
		}

		stdout := &bytes.Buffer{}
		stderr := &bytes.Buffer{}

		cmd := cli.Command{
			Name:                  programName,
			Version:               cfg.version,
			Writer:                stdout,
			ErrWriter:             stderr,
			Description:           cfg.longDescription,
			Usage:                 cfg.description,
			EnableShellCompletion: cfg.enableShellCompletion,
			Flags:                 flags,
			Action: func(ctx context.Context, cmd *cli.Command) error {
				tomlFiles := cmd.StringSlice(cfg.loadConfigFlagName)
				for _, file := range tomlFiles {
					source, err := NewTomlFileSource("toml", file)
					if err != nil {
						return err
					}
					tomlSources = append(tomlSources, source)
				}
				return nil
			},
		}
		err = cmd.Run(context.Background(), os.Args)
		if err != nil {
			if stdout.Len() > 0 {
				return errors.New(err.Error() + "\n\n" + stdout.String())
			}
			return err
		}
		if stdout.Len() > 0 { // help was requested -> return an error so that we can exit
			return &helpRequestedError{
				helpText: stdout.String(),
			}
		}
	}

	config, err := NewStructConfigurator(configPointer, tomlSources)
	if err != nil {
		return err
	}
	flags := config.Flags()
	if loadConfigFlag != nil {
		flags = append(flags, loadConfigFlag)
	}
	if duplicate := firstDuplicateFlagName(flags); duplicate != "" {
		return fmt.Errorf("duplicate flag: --%s", duplicate)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd := cli.Command{
		Name:                  programName,
		Version:               cfg.version,
		Description:           cfg.description,
		Writer:                stdout,
		ErrWriter:             stderr,
		Usage:                 cfg.longDescription,
		EnableShellCompletion: cfg.enableShellCompletion,

		Flags: flags,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			config.Apply(cmd)
			return nil
		},
	}

	err = cmd.Run(context.Background(), os.Args)
	if err != nil {
		if stdout.Len() > 0 {
			return errors.New(strings.TrimSpace(err.Error() + "\n\n" + stdout.String()))
		}
		return err
	}
	if stdout.Len() > 0 { // help was requested -> return an error so that we can exit
		return &helpRequestedError{
			helpText: strings.TrimSpace(stdout.String()),
		}
	}
	return nil
}

func firstDuplicateFlagName(flags []cli.Flag) string {
	seen := make(map[string]bool)
	for _, flag := range flags {
		for _, name := range flag.Names() {
			isDuplicate, ok := seen[name]
			if ok && isDuplicate {
				return name
			}
			seen[name] = true
		}
	}
	return ""
}
