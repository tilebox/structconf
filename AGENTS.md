# AGENTS.md

## Commands
- Build: `go build ./...`
- Test all: `go test ./...`
- Test single: `go test -run TestName ./...`
- Lint: `golangci-lint run ./...`
- Format touched Go files with `gofmt`/`goimports`; the linter also enables `gci`, `gofmt`, `gofumpt`, and `goimports` formatters.

## Repo Architecture
- This is a single-package Go module: `github.com/tilebox/structconf`.
- `structconf.go`: Public loading API (`Load`, `LoadArgs`, `MustLoad`, `MustLoadArgs`), functional options, subcommand helpers (`BindCommand`, `NewCommand`), duplicate flag detection, and the two-pass TOML load flow.
- `config.go`: Reflection-based struct walker that turns exported struct fields into `urfave/cli/v3` flags and applies parsed values back into the config struct.
- `tags.go`: Struct tag parsing and default name derivation for flags, env vars, TOML/YAML/JSON keys, aliases, global fields, secrets, defaults, and help text.
- `toml.go`: TOML file loading and `cli.ValueSource` adapters used to feed TOML values into the same precedence chain as env vars/defaults.
- `validate.go`: Default validation using `go-playground/validator/v10`, with user-facing error messages.
- `marshal.go`: Config marshaling helpers (`MarshalAsMap`, `MarshalAsSlogDict`) and secret redaction.
- `examples`: Runnable examples for each major feature; keep examples small and aligned with README snippets.

## Core Behavior And Contracts
- Source precedence is CLI flags first, then TOML config files, then environment variables, then `default` tags.
- Exported fields get generated names by default: kebab-case flags/TOML/YAML keys, screaming snake env vars, and lower camel JSON names.
- Nested structs compose parent names unless a field is tagged `global:"true"`.
- Fields tagged `flag:"-"` are not configurable; analogous `env:"-"` and `toml:"-"` disable those sources.
- Supported config field types are `string`, `[]string`, signed/unsigned integer widths, floats, `bool`, and `time.Duration`.
- `LoadArgs`/`BindCommand` run validation after values are applied; custom validators replace the default validator.

## Design Patterns And Paradigms
- Public APIs use functional options (`WithVersion`, `WithDescription`, `WithValidator`, etc.).
- Keep the reflection pipeline centralized in `NewStructConfigurator`, `recurseStruct`, and `processField`; avoid one-off parsing paths that bypass the shared value-source precedence behavior.
- Prefer adding behavior at the source of truth (`config.go`, `tags.go`, or `toml.go`) rather than wrapping public APIs for special cases.
- Return errors with useful context and `%w` when wrapping underlying failures.
- Keep generated CLI help/error text stable where tests assert on it.

## Code Style
- Use `stretchr/testify` (`require` for setup/fatal checks, `assert` for value comparisons).
- Prefer table-driven tests for multi-source or multi-type behavior.
- Keep imports grouped and formatted by Go tooling.
- Respect struct tag order enforced by lint: `flag`, `env`, `default`, `secret`, `toml`, `json`, `validate`, `global`, `help`.
- Avoid new package-level globals and `init` functions unless there is a strong reason; `tags.go` currently has an intentional `init` for `strcase` initialism configuration.
- Examples may print to stdout; library code should not, except for existing `MustLoad*` error/help handling.

## Typical Development Flow
1. Make the smallest focused change in the root package and update README/examples/changelog when behavior changes.
2. Add or update focused tests in `structconf_test.go` for precedence, tag handling, validation, or CLI behavior.
3. Run `go test ./...` for behavior changes.
4. Run `go build ./...` when examples or public APIs change.
5. Run `golangci-lint run ./...` before handing off larger changes or anything likely to touch lint-sensitive code.
