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

type ConfigValidator func(cmd *cli.Command, configPointer any) error

type cliOptions struct {
	version               string
	description           string
	longDescription       string
	loadConfigFlagName    string
	enableShellCompletion bool
	commandOptions        commandOptions
}

type commandOptions struct {
	validator              ConfigValidator
	disableImplicitEnvVars bool
	skipSources            bool // The config-file discovery pass parses CLI flags only.
}

type (
	Option        interface{ apply(*cliOptions) }
	CommandOption interface{ applyCommand(*commandOptions) }
	loadOption    func(*cliOptions)
	commandOption func(*commandOptions)
)

func (o loadOption) apply(opts *cliOptions)               { o(opts) }
func (o commandOption) apply(opts *cliOptions)            { o(&opts.commandOptions) }
func (o commandOption) applyCommand(opts *commandOptions) { o(opts) }

// WithDisableImplicitEnvVars disables inferred environment names. Explicit,
// nonempty env tags still bind environment variables. It works with both Load
// and command binding.
func WithDisableImplicitEnvVars() interface {
	Option
	CommandOption
} {
	return commandOption(func(opts *commandOptions) { opts.disableImplicitEnvVars = true })
}

func WithVersion(version string) Option {
	return loadOption(func(opts *cliOptions) {
		opts.version = version
	})
}

func WithDescription(description string) Option {
	return loadOption(func(opts *cliOptions) {
		opts.description = description
	})
}

func WithLongDescription(usage string) Option {
	return loadOption(func(opts *cliOptions) {
		opts.longDescription = usage
	})
}

func WithShellCompletions() Option {
	return loadOption(func(opts *cliOptions) {
		opts.enableShellCompletion = true
	})
}

func WithDefaultLoadConfigFlag() Option {
	return WithLoadConfigFlag("load-config")
}

func WithLoadConfigFlag(flagName string) Option {
	return loadOption(func(opts *cliOptions) {
		opts.loadConfigFlagName = flagName
	})
}

// WithValidator replaces tag validation for loading or command binding.
func WithValidator(validator ConfigValidator) interface {
	Option
	CommandOption
} {
	return commandOption(func(opts *commandOptions) {
		opts.validator = validator
	})
}

// WithCommandValidator is the command-only alias of WithValidator.
func WithCommandValidator(validator ConfigValidator) CommandOption {
	return WithValidator(validator)
}

// WithDisableValidation disables validation for loading or command binding.
func WithDisableValidation() interface {
	Option
	CommandOption
} {
	return WithValidator(func(*cli.Command, any) error { return nil })
}

// WithDisableCommandValidation is the command-only alias of WithDisableValidation.
func WithDisableCommandValidation() CommandOption {
	return WithDisableValidation()
}

// MustLoad is like Load, but if it fails, it prints the error to stderr and exits
// with a non-zero exit code.
func MustLoad(configPointer any, programName string, opts ...Option) {
	MustLoadArgs(configPointer, programName, os.Args, opts...)
}

// MustLoadArgs is like LoadArgs, but if it fails, it prints the error to stderr and exits
// with a non-zero exit code.
func MustLoadArgs(configPointer any, programName string, args []string, opts ...Option) {
	err := LoadArgs(configPointer, programName, args, opts...)
	if err != nil {
		helpRequested := &helpRequestedError{}
		if errors.As(err, &helpRequested) {
			fmt.Println(helpRequested.helpText) //nolint:forbidigo
			os.Exit(0)                          // no error, since we requested help
		}

		_, printErr := fmt.Fprintln(os.Stderr, err.Error())
		if printErr != nil {
			fmt.Println(err.Error()) //nolint:forbidigo    // we couldn't print to stderr, so let's print to stdout instead
		}
		os.Exit(1)
	}
}

// Load loads the given config struct.
//
// It loads the config from the following sources in the given order:
// 1. command line flags and arguments
// 2. TOML config files (when WithLoadConfigFlag is enabled)
// 3. environment variables
// 4. default values defined in the field tags
//
// It then runs the configured validator, which uses the validate tag in config fields by default.
// The returned error is suitable to be printed to the user.
func Load(configPointer any, programName string, opts ...Option) error {
	return LoadArgs(configPointer, programName, os.Args, opts...)
}

// LoadArgs is like Load, but allows explicitly providing the CLI args.
// When positional bindings exist, it rejects any unconsumed arguments.
func LoadArgs(configPointer any, programName string, args []string, opts ...Option) error {
	cfg := &cliOptions{
		commandOptions: commandOptions{
			validator: validate, // default validator
		},
	}
	for _, opt := range opts {
		opt.apply(cfg)
	}

	tomlSources := make([]cli.MapSource, 0)

	var loadConfigFlag cli.Flag
	if cfg.loadConfigFlagName != "" {
		loadConfigFlag = &cli.StringSliceFlag{
			Name:  cfg.loadConfigFlagName,
			Usage: "Load configuration from TOML files",
		}

		discoveryOptions := cfg.commandOptions
		discoveryOptions.skipSources = true
		config, err := newStructConfigurator(configPointer, nil, discoveryOptions)
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

		err = cmd.Run(context.Background(), args)
		if err != nil {
			if stdout.Len() > 0 {
				return fmt.Errorf("%w\n\n%s", err, stdout.String())
			}
			return err
		}

		// Help and completion are rendered again by the final command, which
		// includes positional arguments and all source metadata.
	}

	config, err := newStructConfigurator(configPointer, tomlSources, cfg.commandOptions)
	if err != nil {
		return err
	}

	flags := config.Flags()
	if loadConfigFlag != nil {
		// Do not reuse the discovery flag's parsed values or occurrence count.
		flags = append(flags, &cli.StringSliceFlag{
			Name:  cfg.loadConfigFlagName,
			Usage: "Load configuration from TOML files",
		})
	}

	if duplicate := firstDuplicateFlagName(flags); duplicate != "" {
		return fmt.Errorf("duplicate flag: --%s", duplicate)
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd := &cli.Command{
		Name:                  programName,
		Version:               cfg.version,
		Writer:                stdout,
		ErrWriter:             stderr,
		Description:           cfg.longDescription,
		Usage:                 cfg.description,
		EnableShellCompletion: cfg.enableShellCompletion,

		Flags:     flags,
		Arguments: config.Arguments(),
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if len(cmd.Arguments) > 0 && cmd.NArg() > 0 {
				return &ValidationError{Issues: []ValidationIssue{{
					Message: fmt.Sprintf("unexpected positional arguments: got %d extra", cmd.NArg()),
				}}}
			}
			config.Apply(cmd)
			return nil
		},
	}

	err = cmd.Run(context.Background(), args)
	if err != nil {
		if stdout.Len() > 0 {
			return fmt.Errorf("%w\n\n%s", err, strings.TrimSpace(stdout.String()))
		}
		return err
	}

	if stdout.Len() > 0 { // help was requested -> return an error so that we can exit
		return &helpRequestedError{
			helpText: strings.TrimSpace(stdout.String()),
		}
	}

	return cfg.commandOptions.validator(cmd, configPointer)
}

// NewCommand creates a urfave/cli command and binds the given config struct to it.
//
// When the command is executed, the config is loaded from flags, arguments, env vars and default values,
// then the configured validator is run before the optional action is executed.
//
// The WithLoadConfigFlag option is not currently supported for BindCommand/NewCommand.
func NewCommand(configPointer any, commandName string, action cli.ActionFunc, opts ...CommandOption) (*cli.Command, error) {
	cmd := &cli.Command{
		Name:   commandName,
		Action: action,
	}

	err := BindCommand(cmd, configPointer, opts...)
	if err != nil {
		return nil, err
	}

	return cmd, nil
}

// BindCommand binds the given config struct to an existing urfave/cli command.
//
// It appends reflected flags and arguments to the command and wraps the command's Action so that config
// loading and the configured validator are run before the existing Action.
// Unconsumed arguments remain available to the action through cmd.Args().
//
// The WithLoadConfigFlag option is not currently supported for BindCommand/NewCommand.
func BindCommand(command *cli.Command, configPointer any, opts ...CommandOption) error {
	cfg := &commandOptions{
		validator: validate, // default validator
	}
	for _, opt := range opts {
		opt.applyCommand(cfg)
	}

	config, err := newStructConfigurator(configPointer, nil, *cfg)
	if err != nil {
		return err
	}

	flags := append([]cli.Flag{}, command.Flags...)
	flags = append(flags, config.Flags()...)

	if duplicate := firstDuplicateFlagName(flags); duplicate != "" {
		return fmt.Errorf("duplicate flag: --%s", duplicate)
	}

	command.Flags = flags
	command.Arguments = append(command.Arguments, config.Arguments()...)

	wrappedAction := command.Action
	command.Action = func(ctx context.Context, cmd *cli.Command) error {
		config.Apply(cmd)
		if err := cfg.validator(command, configPointer); err != nil {
			return err
		}

		if wrappedAction == nil {
			return nil
		}

		return wrappedAction(ctx, cmd)
	}

	return nil
}

type helpRequestedError struct {
	helpText string
}

func (e *helpRequestedError) Error() string {
	return e.helpText
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
