package main

import (
	"github.com/tilebox/structconf"
)

type AppConfig struct {
	// must be set (not empty)
	Host string `validate:"required" help:"Hostname (required)"`

	// must be an integer between 1 and 65535
	Port int `validate:"gte=1,lte=65535" default:"8080" help:"Server port"`

	// If set, it must be a valid path to a directory
	Path string `validate:"omitempty,dir" help:"A valid path"`

	// must be one of (case insensitive): DEBUG, INFO, WARN, ERROR
	LogLevel string `default:"INFO" validate:"oneofci=DEBUG INFO WARN ERROR" help:"Log level"`
}

// usage: ./app --log-level=DEBUG --port=8080 --host=localhost --path=/tmp/
func main() {
	cfg := &AppConfig{}
	structconf.MustLoadAndValidate(cfg, "app", structconf.WithVersion("1.0.0"))
}
