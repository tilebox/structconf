# Examples

Run these commands from the repository root. Each directory contains a standalone program; the examples print results without contacting external services.

## Basic configuration

| Example | Shows | Run |
| --- | --- | --- |
| [01_simple_cli](01_simple_cli) | Flags, inferred environment variables, generated help | `go run ./examples/01_simple_cli --greet --name World` |
| [02_default_values](02_default_values) | Defaults and help text | `go run ./examples/02_default_values --greet` |
| [03_nested](03_nested) | Nested field names | `go run ./examples/03_nested --server-host localhost --server-port 8080` |
| [04_toml](04_toml) | TOML configuration with CLI overrides | `go run ./examples/04_toml --load-config examples/04_toml/config.toml --log-level DEBUG` |
| [05_override](05_override) | Custom source names and excluded fields | `go run ./examples/05_override --level DEBUG` |
| [06_global](06_global) | Names without parent prefixes | `go run ./examples/06_global --name Tilebox` |
| [07_marshal](07_marshal) | Maps, slog, and secret redaction | `go run ./examples/07_marshal --database-user demo --database-password example-password` |
| [08_validation](08_validation) | Required fields, ranges, and allowed values | `go run ./examples/08_validation --host localhost --port 8080 --path /tmp` |
| [09_subcommands](09_subcommands) | `NewCommand`, `BindCommand`, and shell completion | `go run ./examples/09_subcommands sum --left 10 --right 5` |

## Repeated flags and typed slices

[10_slices](10_slices) combines comma-separated origins and integers with literal repeated filters:

```bash
go run ./examples/10_slices --filter "name = 'a,b'" --filter "cloud < 20" --counts 2,5
ALLOWED_ORIGINS=https://app.example.com go run ./examples/10_slices
go run ./examples/10_slices --load-config examples/10_slices/config.toml
```

The explicit `env:"ALLOWED_ORIGINS"` binding works with `WithDisableImplicitEnvVars()`. Fields without an explicit environment name do not bind environment variables.

Comma-separated mode splits on commas without CSV quoting or whitespace trimming. Repeated-flag mode treats each occurrence as one element, including commas and empty strings. Its environment/default source also supplies one literal element; use a TOML array for multiple fallback elements. The first CLI occurrence replaces the entire fallback slice.

## Positional arguments

[11_arguments](11_arguments) uses a required name (`nargs:"1"`) and an optional count (`nargs:"?"`) with a default:

```bash
go run ./examples/11_arguments Tilebox
go run ./examples/11_arguments --greeting Hi --loud Tilebox 3
```

| `nargs` | Values consumed | Field type |
| --- | --- | --- |
| `?` | Zero or one | Scalar or scalar pointer |
| `1` | Exactly one | Scalar, scalar pointer, or slice |
| `2`, `3`, … | Exactly that many | Slice |
| `*` | Zero or more | Slice |
| `+` | One or more | Slice |
| Omitted | Zero or one; slices split that token on commas | Scalar, scalar pointer, or slice |

Argument indices order fields, not tokens. For example, a pair at `arg:"0"` consumes two tokens before `arg:"1"`:

```go
type RangeArgs struct {
    Bounds []int   `arg:"0" nargs:"2"`
    Label  *string `arg:"1" nargs:"?"`
}
// 10 25 window → Bounds: []int{10, 25}, Label: pointer to "window"
```

Explicit `nargs` preserves each token literally. An omitted argument can use fallback sources, but a partially supplied group cannot fill its missing elements from a fallback. The resolved length must satisfy `nargs`. Required fields must precede optional fields, and a variadic (`*` or `+`) field must be last.

For compatibility, non-pointer arguments without `nargs` retain preinitialized values if no source supplies input. Pointers and explicit `nargs` bindings reset on each parse. Use explicit cardinality for new commands.

[13_forwarding](13_forwarding) combines a required target with a variadic command. Urfave's `StopOnNthArg: new(1)` stops parsing flags after the target, so flags belonging to the child command remain literal arguments:

```bash
go run ./examples/13_forwarding -n 2 local echo --literal-flag "a,b" ""
```

```text
Target: local
Workers: 2
Command: ["echo" "--literal-flag" "a,b" ""]
```

The example only prints the command. A real runner would pass the argument slice to its execution API without joining it into a shell command. `Load` / `LoadArgs` reject leftovers when positional bindings exist; `BindCommand` / `NewCommand` leave leftovers in `cmd.Args()` for the action to handle.

## Partial updates and structured errors

[12_updates](12_updates) uses scalar pointers for partial updates, a file-completion flag, and a custom validator. Omitted fields stay nil; explicitly empty strings and false booleans still request an update:

```bash
go run ./examples/12_updates update --summary "" --enabled=false my-workflow
go run ./examples/12_updates update --summary-file README.md my-workflow
```

```text
Set my-workflow summary to ""
Set my-workflow enabled to false
```

The output above is from the first command. The second reads the file in the action: `file:"true"` only supplies completion metadata. Pointer defaults would make an omitted input non-nil, so these update fields deliberately have no defaults.

Try conflicting inputs to see focused help from `ValidationError.Issues`:

```bash
go run ./examples/12_updates update --summary text --summary-file README.md my-workflow
```

```text
choose either --summary or --summary-file
  --summary: New summary; an empty string clears it
  --summary-file: Read the summary from a file
```

The custom validator calls `structconf.Validate` first to keep tag validation. It then creates errors with `NewValidationError(message, config, paths...)`, using Go field paths such as `SummaryFile` or `Server.Port`. The renderer recognizes wrapped structured errors with `errors.As` and leaves other errors intact.

For inputs defined outside structconf, construct the same error type directly:

```go
return &structconf.ValidationError{
    Issues: []structconf.ValidationIssue{{
        Message: "authentication is required",
        Fields: []structconf.FieldInfo{{
            Name: "--api-key",
            Help: "API key used to authenticate requests",
        }},
    }},
}
```

`Describe(config)` exposes field paths, CLI names, and help without loading values; it also accepts typed nil pointers. Use it for command documentation or other metadata consumers. Neither field metadata nor default validation messages include input values. Custom messages and native urfave parsing errors may include them.

## Shell completion

The subcommand and update examples enable completion on their root command. For a standalone loader, pass `WithShellCompletions()`. After building your application as `app`, install its completion script:

```bash
# Bash
source <(app completion bash)

# Zsh
source <(app completion zsh)

# Fish
app completion fish > ~/.config/fish/completions/app.fish
```
