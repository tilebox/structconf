# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.6.0] - 2026-11-09

### Added

- Added `slice:"comma-separated"` (the default) and `slice:"repeated-flag"` for per-field slice parsing, plus loading slices of all supported scalar types. Repeated flags preserve literal commas; native TOML arrays preserve element boundaries in both modes.
- Added scalar pointer fields to distinguish omitted values from explicit zero values. Defaults populate pointers too; nil pointers are omitted when marshaling.
- Added positional `nargs` cardinality: `?`, `*`, `+`, and positive integer counts. Omitting `nargs` retains the existing single-token/fallback behavior.
- Added `WithDisableImplicitEnvVars()` for loading and command binding, retaining only explicitly named environment sources.
- Added `FieldInfo`, `Describe`, `ValidationIssue`, `ValidationError`, `NewValidationError`, and `Validate` for shared field metadata and application-defined validation errors.
- Added `file:"true"` on string flags, including scalar pointers, for urfave file completion.

### Changed

- Requires Go 1.26 or later; CI tests Go 1.26 and 1.27.
- `Load` and `LoadArgs` now reject leftover tokens when positional bindings exist. `BindCommand` and `NewCommand` still expose leftovers through `cmd.Args()`.
- Required positional bindings cannot follow fields with omitted `nargs`, just as they cannot follow `nargs:"?"`. Use `nargs:"1"` for preceding required fields; `validate:"required"` does not determine positional cardinality.
- `WithValidator` and `WithDisableValidation` now work with both loading and command binding. Their command-only counterparts remain supported as aliases.
- `MarshalAsSlogDict` now preserves leaf value types using `slog.Any`. JSON logs contain numbers, booleans, and arrays instead of stringified values; consumers expecting strings must adapt. Durations remain strings and secrets remain redacted.
- Tag validation and structconf-owned positional/slice-source errors now expose structured field information. Tag errors name the affected input and explain required values, choices, lengths, bounds, and common formats without echoing rejected values. Urfave flag errors retain their native types.
- Removed unused internal JSON/YAML tag handling; neither format was supported for loading or marshaling.
- `Option` and `CommandOption` are now sealed interfaces so `WithDisableImplicitEnvVars()` can be passed to either API. Existing option constructors and option slices remain supported.

### Fixed

- Pointer positional arguments work with urfave's typed getters and report scalar input types. Pointer flags without declared defaults no longer advertise zero-value defaults.
- Omitted non-pointer positional arguments without `nargs` preserve initialized values when no fallback is available.
- Config-file discovery no longer parses environment values before TOML sources are available, and help includes the final command's argument and source metadata.
- Slice flags report occurrence counts and honor per-flag help stringers, including inherited flags and source fallbacks.
- Marshaling skips nil nested struct pointers without allocating or modifying the config.
- Invalid indexed validation paths no longer resolve to unrelated CLI bindings. Validation resolves field metadata once per call.
- Fixed `MarshalAsMap` and `MarshalAsSlogDict` failing on slices. Elements use the same primitive conversions and string redaction as scalar fields, including in nested slices.
- Aligned positional arguments with flags when reading empty source values: numeric and duration fields fall back, strings and string slices are cleared, and booleans become `false`.

## [0.5.0] - 2026-05-20

### Added

- Added positional CLI argument binding via the `arg` struct tag.

## [0.4.0] - 2026-05-20

### Added

- Added support for configuring `[]string` fields from comma-separated CLI flags, environment variables, and default values, plus native TOML string arrays.

## [0.3.0] - 2026-05-12

### Added

- Added custom validator registration via `WithValidateFunc`.

## [0.2.0] - 2026-03-13

### Added

- Added composable command binding helpers for subcommand CLIs via `BindCommand` and `NewCommand`.
- Added a subcommands example.Now

## [0.1.0] - 2026-01-02

### Added

- Added initial struct tag based configuration loading from CLI flags, environment variables, TOML config files, and default values.
- Added support for nested structs, field tags, generated help output, validation, and marshaling.
- Added examples covering basic CLI usage, defaults, nested structs, TOML loading, overrides, globals, marshaling, and validation.

### Fixed

- Fixed integer parsing to use the correct bit size for signed and unsigned integer fields.

[Unreleased]: https://github.com/tilebox/structconf/compare/v0.6.0...HEAD
[0.6.0]: https://github.com/tilebox/structconf/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/tilebox/structconf/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/tilebox/structconf/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/tilebox/structconf/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/tilebox/structconf/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/tilebox/structconf/releases/tag/v0.1.0
