package main

import (
	"fmt"

	"github.com/tilebox/structconf"
)

type AppConfig struct {
	Deeply DeeplyConfig
}

type DeeplyConfig struct {
	Nested NestedConfig
}

type NestedConfig struct {
	Name string `global:"true"` // will be --name (and $NAME) instead of --deeply-nested-name and $DEEPLY_NESTED_NAME
}

// usage: ./app --load-config database.toml --log-level=debug
func main() {
	cfg := &AppConfig{}
	structconf.MustLoadAndValidate(cfg,
		"app",
		structconf.WithVersion("1.0.0"),
	)

	fmt.Println(cfg.Deeply.Nested.Name)
}
