package main

import (
	"fmt"

	"github.com/tilebox/structconf"
)

type AppConfig struct {
	AllowedOrigins []string `flag:"allowed-origins" env:"ALLOWED_ORIGINS" toml:"allowed-origins" validate:"omitempty,dive,url" help:"comma-separated list of allowed CORS origins"`
}

// usage: ./app --allowed-origins=https://app.example.com,https://admin.example.com
// or: ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com ./app
// or with TOML: allowed-origins = ["https://app.example.com", "https://admin.example.com"]
func main() {
	cfg := &AppConfig{}
	structconf.MustLoad(cfg, "app", structconf.WithVersion("1.0.0"), structconf.WithLoadConfigFlag("load-config"))

	fmt.Printf("%v\n", cfg.AllowedOrigins)
}
