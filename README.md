# structconf

Define your CLI inputs in a Go struct. A command like this:

```bash
./my-program --limit 50 --collections optical,radar --db-user "dummy" my-dataset
```

maps to these fields:

```go
type Args struct {
    Dataset     string   `arg:"0" nargs:"1"` // my-dataset
    Limit       int      `default:"20"`  // 50
    Collections []string  // ["optical", "radar"]
    APIKey      string   `env:"API_KEY"`  // read from $API_KEY if not passed on the CLI
    DB          Database
}

type Database struct {
    User string // "dummy", from --db-user
}
```

Or put the same values in `config.toml`:

```toml
api-key = "example-key"
dataset = "my-dataset"
limit = 50
collections = ["optical", "radar"]

[db]
user = "dummy"
```

Enable file loading with `WithDefaultLoadConfigFlag()`, then run:

```bash
./my-program --load-config=config.toml
```

structconf handles parsing, help, and validation using [urfave/cli](https://github.com/urfave/cli) and [go-playground/validator](https://github.com/go-playground/validator).

## Struct tags

Tags are optional. By default, structconf uses exported field names for flags, environment variables, and TOML keys. Nested fields include their parents' names.

| Tag | Example | Purpose |
| --- | --- | --- |
| `flag` | `flag:"output"` | Override the flag name. `flag:"-"` excludes a field from configuration. |
| `arg` | `arg:"0"` | Read a positional argument instead of a flag. Number fields starting at zero. |
| `alias` | `alias:"-n"` | Add flag aliases, comma-separated and prefixed with `-`. |
| `env` | `env:"API_KEY"` | Override the environment variable name. `env:"-"` disables this source. |
| `default` | `default:"10"` | Use this value when no input source supplies one. |
| `toml` | `toml:"api-key"` | Override the TOML key. `toml:"-"` disables this source. |
| `help` | `help:"Maximum results"` | Describe the field in CLI help and field metadata. |
| `validate` | `validate:"gte=1"` | Apply go-playground/validator constraints after loading. |
| `secret` | `secret:"true"` | Redact string values in structconf's marshaling helpers. |
| `global` | `global:"true"` | Leave parent prefixes out of the field's names. |
| `slice` | `slice:"repeated-flag"` | Append one value each time the flag is passed, without splitting commas. Defaults to `comma-separated`. |
| `nargs` | `nargs:"+"` | Set how many positional values to accept: `?`, `*`, `+`, or a positive integer. |
| `file` | `file:"true"` | Enable file completion for `string` and `*string` flags. |

Fields can be strings, booleans, signed and unsigned integers, floats, or `time.Duration`, including pointers and slices of these types. Use nested structs to group configuration.

## Load configuration

Install with Go 1.26 or later:

```bash
go get github.com/tilebox/structconf
```

Define a struct and pass its pointer to `MustLoad`. Save this as `main.go` in your application:

```go
package main

import (
    "fmt"

    "github.com/tilebox/structconf"
)

type Config struct {
    Endpoint string `default:"https://api.example.com" validate:"url" help:"API endpoint"`
    Limit    int    `default:"20" validate:"gte=1" help:"Maximum results"`
}

func main() {
    cfg := &Config{}
    structconf.MustLoad(cfg, "query",
        structconf.WithDescription("Query records"),
        structconf.WithDefaultLoadConfigFlag(),
    )
    fmt.Printf("Query %s, limit %d\n", cfg.Endpoint, cfg.Limit)
}
```

Run it with flags or environment variables. To try file loading, create `config.toml` containing `limit = 30`:

```bash
go run . --limit 50
LIMIT=50 go run .
go run . --load-config config.toml --limit 50
go run . --help
```

When a field has more than one source, the first one wins: **CLI → TOML files → environment variables → defaults**. Flags and TOML keys use kebab-case; environment variables use SCREAMING_SNAKE_CASE. For example, `Server.Port` becomes `--server-port`, `SERVER_PORT`, and `port` under TOML's `[server]` table.

Use `Load` if you want to handle errors yourself instead of printing them and exiting. For tests or calls from Go code, `LoadArgs` and `MustLoadArgs` take an argument slice that includes the program name.

Environment variables are read automatically. Pass `WithDisableImplicitEnvVars()` to read only variables named in nonempty `env` tags. This works with both `Load` and command binding.

## Bind commands

Use `BindCommand` with an existing urfave command. It adds the inputs from your struct, then loads and validates them before the action runs:

```go
type UpdateArgs struct {
    Workflow string  `arg:"0" nargs:"1" help:"Workflow to update"`
    Summary  *string `help:"New summary; an empty string clears it"`
    Enabled  *bool   `help:"Enable or disable the workflow"`
}

func updateCommand() (*cli.Command, error) {
    args := &UpdateArgs{}
    cmd := &cli.Command{
        Name:  "update",
        Usage: "Update a workflow",
        Action: func(_ context.Context, _ *cli.Command) error {
            if args.Summary != nil {
                fmt.Printf("Set %s summary to %q\n", args.Workflow, *args.Summary)
            }
            if args.Enabled != nil {
                fmt.Printf("Set %s enabled to %t\n", args.Workflow, *args.Enabled)
            }
            return nil
        },
    }
    err := structconf.BindCommand(cmd, args, structconf.WithDisableImplicitEnvVars())
    return cmd, err
}
```

Add the command to a root command's `Commands` slice and run it with `root.Run(ctx, os.Args)`. `NewCommand(args, name, action, options...)` creates and binds a command in one call. See the [runnable subcommand example](examples/09_subcommands).

Use pointers when an omitted flag should mean “leave this field alone.” Passing `--summary ""` or `--enabled=false` gives you a non-nil pointer, so you can clear a string or turn something off. Defaults also set pointers, so leave defaults off fields used for partial updates.

Command binding supports flags, arguments, environment variables, and defaults. TOML loading is available through `Load` / `LoadArgs`, not command binding. Set shell completion on the urfave root command, or use `WithShellCompletions()` with `Load`.

## Slices and positional arguments

```go
type QueryArgs struct {
    Dataset     string   `arg:"0" nargs:"1"`
    Collections []string `flag:"collections"`
    Filters     []string `flag:"filter" slice:"repeated-flag"`
    Counts      []int    `flag:"counts"`
}
// query --collections a,b --filter "name = 'a,b'" --filter "cloud < 20" --counts 3,7 dataset
```

Slices split commas by default. Use `slice:"repeated-flag"` when each flag should add one value, even if it contains commas. This is useful for filter expressions. The first flag replaces any values from TOML, environment variables, or defaults. Repeating it appends in repeated-flag mode and replaces in comma-separated mode. TOML arrays keep each element intact in either mode.

For positional arguments, use `nargs:"1"` for one required value, `?` for an optional value, `*` for zero or more values, or `+` for one or more. Use a slice for `*`, `+`, or a fixed count above one. Number fields from zero without gaps, put required fields before optional ones, and leave `*` or `+` for the last field. Without `nargs`, a field reads at most one argument and splits commas if it's a slice.

When you define positional arguments, `Load` rejects extra arguments. `BindCommand` and `NewCommand` leave them in `cmd.Args()` for your action to handle. See the [argument and forwarding examples](examples/README.md#positional-arguments).

## Validation and errors

`validate` tags run by default. Use `WithValidator` to replace validation, or `WithDisableValidation` to skip it. Both work with loading and command binding.

Validation messages use flag or positional argument names, falling back to Go field
paths for fields without a CLI binding. Common constraints explain what to supply:
`--name must contain at least 1 character`, `--limit must be at most 100`, or
`<id> must be a valid UUID`. Length constraints count characters in strings and
items in collections; numeric constraints describe the allowed value. Messages
do not include rejected input values. `Rule` and `Param` remain available in each
issue for applications that need to inspect the constraint.

Call `structconf.Validate` inside your validator to keep the tag checks, then add your own:

```go
type InputArgs struct {
    Input     *string
    InputFile *string `file:"true"`
}

func validateInput(_ *cli.Command, config any) error {
    if err := structconf.Validate(config); err != nil {
        return err
    }
    args := config.(*InputArgs)
    if args.Input != nil && args.InputFile != nil {
        return structconf.NewValidationError(
            "choose either --input or --input-file", args, "Input", "InputFile",
        )
    }
    return nil
}
```

Use `errors.As` to find a `*ValidationError`, then read its `Issues` for messages, field names, and help text. You can also construct these errors yourself. If you only need field names and help, call `Describe(config)` without loading any values. The [update example](examples/12_updates) shows how to validate inputs and print errors with help for the affected fields.

Not every error is a `ValidationError`: urfave flag errors and file I/O errors retain their native types. The `secret` tag does not redact arbitrary error messages.

## Logging configuration

Use `MarshalAsMap(config)` to inspect a config or `MarshalAsSlogDict(config, "config")` to log it with slog. Both handle nested structs and slices, skip nil pointers, and redact strings tagged `secret:"true"` without changing the config. Keys follow flag names. structconf does not read JSON or YAML tags or config files. See the [logging example](examples/07_marshal).

## Examples

See [examples/](examples/README.md) for runnable programs covering TOML, subcommands, repeated flags, partial updates, error handling, and command forwarding.
