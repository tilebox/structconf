# structconf

Opinionated struct tag based configuration for go - parse CLI args, environment vars or config files into a unified config struct - based on [urfave/cli](https://github.com/urfave/cli). 

## Installation

```bash
go get github.com/tilebox/structconf
```

## Features

- Load configuration from CLI flags, CLI arguments, environment variables, `.toml` config files or specified default values - or from all of them at once
  - Order of precedence: CLI flags/arguments, config files, environment variables, default values
- By only defining a struct containing all the fields you want to configure
- Structs can be nested within other structs
- Supported data types: `string`, `[]string`, `int`, `int8-64`, `uint`, `uint8-64`, `bool`, `float`, `time.Duration`
- Customize certain fields by adding tags to the struct fields
  - Using the tags `flag`, `arg`, `env`, `default`, `secret`, `toml`, `validate`, `global`, `help`
- Includes input validation using [go-playground/validator](https://github.com/go-playground/validator)
- Help message generated out of the box
- Composable command binding helpers for subcommand CLIs via `BindCommand` / `NewCommand`

## Usage

```go
package main

import (
    "fmt"
    "github.com/tilebox/structconf"
)

type ProgramConfig struct {
    Name  string
    Greet bool
}

func main() {
    cfg := &ProgramConfig{}
    structconf.MustLoad(cfg, "greetings")
    if cfg.Greet {
        fmt.Printf("Hello %s!\n", cfg.Name)
    }
}
```

Produces the following program:

```
$ ./greetings --greet --name "World"
Hello World!
```

Alternatively, also environment variables are read out of the box:

```
$ GREET=true NAME=World ./greetings
Hello World!
```

And also a help message is generated out of the box:

```
$ ./greetings -h

NAME:
   simple_cli - A new cli application

USAGE:
   simple_cli [global options]

VERSION:
   1.0.0

GLOBAL OPTIONS:
   --name value    [$NAME]
   --greet    (default: false) [$GREET]
   --help, -h     show help
   --version, -v  print the version
```

## Features

### Specify default values and help messages

```go
type ProgramConfig struct {
  Name  string `default:"World" help:"Whom to greet"`
  Greet bool   `help:"Whether or not to greet"`
}

func main() {
  cfg := &ProgramConfig{}
  structconf.MustLoad(cfg,
    "greetings",
    structconf.WithVersion("1.0.0"),
    structconf.WithDescription("Print a greeting"),
    structconf.WithLongDescription("A CLI for printing a greeting to the console"),
  )

  if cfg.Greet {
    fmt.Printf("Hello %s!\n", cfg.Name)
  }
}

```

```bash
$ ./greetings -h

NAME:
   greetings - Print a greeting

USAGE:
   greetings [global options]

VERSION:
   1.0.0

DESCRIPTION:
   A CLI for printing a greeting to the console

GLOBAL OPTIONS:
   --name value   Whom to greet (default: World) [$NAME]
   --greet        Whether or not to greet (default: false) [$GREET]
   --help, -h     show help
   --version, -v  print the version
```

### Nest structures

```go
type DatabaseConfig struct {
    User     string
    Password string
}

type ServerConfig struct {
    Host string
    Port int
}

type AppConfig struct {
    LogLevel string `default:"INFO"`

    Server   ServerConfig
    Database DatabaseConfig
}

func main() {
    cfg := &AppConfig{}
    structconf.MustLoad(cfg, "app")

    fmt.Printf("%v", cfg)
}
```

```
$ ./app --database-user=myuser --database-password=mypassword --server-host=localhost --server-port=8080 --log-level=DEBUG

&{DEBUG {localhost 8080} {myuser mypassword}}
```

### Load configuration from TOML files

```go
type DatabaseConfig struct {
    User     string
    Password string
}
type AppConfig struct {
    LogLevel string `default:"INFO"`
    Database DatabaseConfig
}

func main() {
    cfg := &AppConfig{}
    structconf.MustLoad(cfg,
        "app",
        // adds a --load-config flag to load config from TOML files
        structconf.WithLoadConfigFlag("load-config"),
    )

    fmt.Printf("%v", cfg)
}
```

Define a config file

```toml
# database.toml
[database]
user = "myuser"
password = "mypassword"
```

Run the program

```bash
$ ./app --load-config database.toml
&{INFO {myuser mypassword}}
```

### Configure string slices

`[]string` fields are configured with comma-separated values from CLI flags, environment variables and default values.
When loading from TOML, native string arrays are supported as well.

```go
type AppConfig struct {
    AllowedOrigins []string `flag:"allowed-origins" env:"ALLOWED_ORIGINS" toml:"allowed-origins" help:"Allowed CORS origins"`
}

func main() {
    cfg := &AppConfig{}
    structconf.MustLoad(cfg, "app", structconf.WithLoadConfigFlag("load-config"))
}
```

```bash
$ ./app --allowed-origins=https://app.example.com,https://admin.example.com
$ ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com ./app
```

```toml
# app.toml
allowed-origins = ["https://app.example.com", "https://admin.example.com"]
```

### Build subcommands

You can bind configs directly to `urfave/cli` commands and compose them as subcommands.

```go
package main

import (
    "context"
    "fmt"
    "os"
    "strings"

    "github.com/tilebox/structconf"
    "github.com/urfave/cli/v3"
)

type GreetConfig struct {
    Name string `default:"World"`
    Loud bool
}

func main() {
    greetCfg := &GreetConfig{}

    greetCmd, err := structconf.NewCommand(greetCfg, "greet", func(ctx context.Context, cmd *cli.Command) error {
        if greetCfg.Loud {
            fmt.Println(strings.ToUpper(greetCfg.Name))
            return nil
        }

        fmt.Println(greetCfg.Name)
        return nil
    })
    if err != nil {
        panic(err)
    }

    root := &cli.Command{
        Name:     "app",
        Commands: []*cli.Command{greetCmd},
    }

    if err := root.Run(context.Background(), os.Args); err != nil {
        panic(err)
    }
}
```

`BindCommand` and `NewCommand` currently support flags, arguments, env vars and default values. `WithLoadConfigFlag` is currently only supported by `Load` / `MustLoad`.

### Bind positional arguments

Use the `arg` tag to bind positional CLI arguments by zero-based index. Argument-bound fields are not also exposed as flags; define either `arg` or `flag` for a field.

```go
type AppConfig struct {
    SomeFlag  string `flag:"some-flag"`
    SecondArg string `arg:"1" default:"world"`
    OtherFlag bool   `flag:"other-flag"`
    FirstArg  int    `arg:"0"`
}

cfg := &AppConfig{}
err := structconf.LoadArgs(cfg, "app", []string{"app", "--some-flag", "hello", "42", "Tilebox", "--other-flag"})
if err != nil {
    panic(err)
}

// cfg.FirstArg == 42
// cfg.SecondArg == "Tilebox"
```

Arguments use the same fallback sources as flags: if a positional argument is omitted, structconf checks TOML config files, then environment variables, then the `default` tag. Argument indexes must start at `0` and be contiguous; duplicate indexes or gaps are reported as errors. Structconf-bound arguments implement `ArgumentMetadata` for tools that need argument name, type name, and usage text.

### Parse custom arg slices

If you need to parse a specific arg slice (for tests or embedding), use `LoadArgs`:

```go
cfg := &AppConfig{}
err := structconf.LoadArgs(cfg, "app", []string{"app", "--log-level", "DEBUG"})
if err != nil {
    panic(err)
}
```

### Shell completion

Enable completion in code:

```go
structconf.MustLoad(cfg, "app", structconf.WithShellCompletions())
```

Then install it in your shell:

```bash
# bash (add to ~/.bashrc)
source <(app completion bash)

# zsh (add to ~/.zshrc)
source <(app completion zsh)

# fish
app completion fish > ~/.config/fish/completions/app.fish
```

### Override auto-generated names for fields

By default, field names are converted to flags, env vars and toml properties using the following rules:

- For flags, (nested) field names are converted to kebab-case, e.g. `MyFieldName` becomes `--my-field-name`
- For env vars, field names are converted to uppercase, e.g. `MyFieldName` becomes `MY_FIELD_NAME`
- For toml properties, field names are converted to kebab-case, e.g. `MyFieldName` becomes `my-field-name`
- Common initialisms are respected, e.g. `MyServerURL` becomes `--my-server-url` or `MY_SERVER_URL`

You can override these default rules at any point by using the `flag`, `arg`, `env` and `toml` tags.

```go
type AppConfig struct {
    // configure using either:
    // --level flag
    // $LOGGING_LEVEL env var
    // log-level toml property
    LogLevel string `flag:"level" env:"LOGGING_LEVEL" toml:"log-level" default:"INFO"`
    
    // will not be configurable at all
    Ignored string `flag:"-" env:"-" toml:"-"`
}
```

### Configure certain fields as global regardless of how deeply nested they are

```go
type AppConfig struct {
    Deeply DeeplyConfig
}

type DeeplyConfig struct {
    Nested NestedConfig
}

type NestedConfig struct {
    Name string `global:"true"` // will be --name (and $NAME) instead of --deeply-nested-name and $DEEPLY_NESTED_NAME
}

func main() {
    cfg := &AppConfig{}
    structconf.MustLoad(cfg, "app")
    fmt.Println(cfg.Deeply.Nested.Name)
}
```

```bash
./app --name Tilebox
Tilebox
```

## Auto-marshalling and log/slog integration

For inspection or logging purposes sometimes it is useful to marshal a whole config struct.
However, when doing so, often sensitive fields need to be redacted.

`structconf` provides marshaling helpers for this use case.


```go
import (
  "fmt"
  "log/slog"
  
  "github.com/tilebox/structconf"
)

type DatabaseConfig struct {
    User     string
    Password string `secret:"true"`
}

type AppConfig struct {
    LogLevel string `default:"DEBUG"`
    Database DatabaseConfig
}


func main() {
    cfg := &AppConfig{}
    structconf.MustLoad(cfg, "app")

    asMap, err := structconf.MarshalAsMap(cfg)
    if err != nil {
        panic(err)
    }
    fmt.Println(asMap)

    // includes an integration with log/slog to convert the config struct to a recursive slog.Group structure
    config, err := structconf.MarshalAsSlogDict(cfg, "config")
    if err != nil {
        panic(err)
    }
    slog.Info("Program config loaded successfully", config)
}
```

```bash
$ ./app --database-user=my-user --database-password=very-secret-password --log-level=INFO
map[database:map[password:ve***rd user:my-user] log-level:DEBUG]
INFO Program config loaded successfully config.log-level=DEBUG config.database.user=my-user config.database.password=ve***rd
```

### Validation

`structconf` includes validation using [go-playground/validator](https://github.com/go-playground/validator)

```go
type AppConfig struct {
    // must be set (not empty) 
    Host string `validate:"required" help:"Hostname (required)"`
    
    // must be an integer between 1 and 65535 
    Port int `validate:"gte=1,lte=65535" default:"8080" help:"Server port"`
    
    // If set, it must be a valid path to a directory 
    Path string `validate:"omitempty,dir" help:"A valid path"`
    
    // must be one of (case insensitive): DEBUG, INFO, WARN, ERROR 
    LogLevel string `default:"INFO" validate:"oneofci=DEBUG INFO WARN ERROR" help:"Log level"`

    // if set, each comma-separated value must be one of (case insensitive): password, oauth, saml, magic_link
    EnabledAuthProviders []string `validate:"omitempty,dive,oneofci=password oauth saml magic_link" help:"Enabled authentication providers"`
}

func main() {
    cfg := &AppConfig{}
    structconf.MustLoad(cfg, "app")
}
```

```bash
$ ./app --port=0 --path=/tmp/
Missing required configuration: AppConfig.Host
Configuration error: Port - gte
```

If you want to load config without running validation, pass `WithDisableValidation`:

```go
type AppConfig struct {
    Host string `validate:"required"`
}

func main() {
    cfg := &AppConfig{}
    structconf.MustLoad(cfg, "app", structconf.WithDisableValidation())
}
```

For configs bound to subcommands with `BindCommand` / `NewCommand`, use `WithDisableCommandValidation` instead.
