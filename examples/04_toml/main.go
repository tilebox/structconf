package main

import (
	"fmt"

	"github.com/tilebox/structconf"
)

type DatabaseConfig struct {
	User     string
	Password string
}
type AppConfig struct {
	LogLevel string `default:"INFO"`
	Database DatabaseConfig
}

// usage: ./app --load-config database.toml --log-level=debug
func main() {
	cfg := &AppConfig{}
	structconf.MustLoadAndValidate(cfg,
		"app",
		structconf.WithVersion("1.0.0"),
		// adds a --load-config flag to load config from TOML files
		structconf.WithLoadConfigFlag("load-config"),
	)

	fmt.Printf("%v", cfg)
}
