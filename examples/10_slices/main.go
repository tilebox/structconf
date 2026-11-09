package main

import (
	"fmt"

	"github.com/tilebox/structconf"
)

type AppConfig struct {
	AllowedOrigins []string `flag:"allowed-origins" env:"ALLOWED_ORIGINS"                                  toml:"allowed-origins"        validate:"omitempty,dive,url" help:"comma-separated list of allowed CORS origins"`
	Filters        []string `flag:"filter"          help:"Filter expression (repeat for multiple filters)" slice:"repeated-flag"`
	Counts         []int    `flag:"counts"          default:"3,7"                                          help:"Comma-separated counts"`
}

// usage: ./app --allowed-origins=https://app.example.com,https://admin.example.com
// or: ./app --filter "name = 'a,b'" --filter "cloud < 20" --counts 2,5
// or: ALLOWED_ORIGINS=https://app.example.com,https://admin.example.com ./app
// or with TOML: allowed-origins = ["https://app.example.com", "https://admin.example.com"]
func main() {
	cfg := &AppConfig{}
	structconf.MustLoad(cfg, "app", structconf.WithVersion("1.0.0"), structconf.WithLoadConfigFlag("load-config"), structconf.WithDisableImplicitEnvVars())

	fmt.Printf("Origins: %q\nFilters: %q\nCounts: %v\n", cfg.AllowedOrigins, cfg.Filters, cfg.Counts)
}
