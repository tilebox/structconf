package main

import (
	"fmt"
	"github.com/tilebox/structconf"
)

type DatabaseConfig struct {
	User     string
	Password string
}

type ServerConfig struct {
	Host string
	Port int
}

type AppConfig struct {
	LogLevel string `default:"DEBUG"`

	Server   ServerConfig
	Database DatabaseConfig
}

// usage: ./app --database-user=myuser --database-password=mypassword --server-host=localhost --server-port=8080 --log-level=DEBUG
func main() {
	cfg := &AppConfig{}
	structconf.MustLoadAndValidate(cfg,
		"app",
		structconf.WithVersion("1.0.0"),
	)

	fmt.Printf("%v", cfg)
}
