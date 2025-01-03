package main

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

// usage: ./app --database-user=my-user --database-password=very-secret-password --log-level=INFO
func main() {
	cfg := &AppConfig{}
	structconf.MustLoadAndValidate(cfg,
		"app",
		structconf.WithVersion("1.0.0"),
	)

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
