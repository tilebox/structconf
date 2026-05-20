# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/tilebox/structconf/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/tilebox/structconf/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/tilebox/structconf/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/tilebox/structconf/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/tilebox/structconf/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/tilebox/structconf/releases/tag/v0.1.0
