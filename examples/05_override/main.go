package main

import (
	"github.com/tilebox/structconf"
)

type AppConfig struct {
	// configure using either:
	// --level flag
	// $LOGGING_LEVEL env var
	// log-level toml property
	LogLevel string `flag:"level" env:"LOGGING_LEVEL" default:"INFO" toml:"log-level"`

	// will not be configurable at all
	Ignored string `flag:"-" env:"-" toml:"-"`
}

// usage: ./app --load-config database.toml --log-level=debug
func main() {
	cfg := &AppConfig{}
	structconf.MustLoadAndValidate(cfg,
		"app",
		structconf.WithVersion("1.0.0"),
	)
}
