package main

import (
	"fmt"

	"github.com/tilebox/structconf"
)

type ProgramConfig struct {
	Name  string
	Greet bool
}

// usage: ./simple_cli --greet --name "World"
func main() {
	cfg := &ProgramConfig{}
	structconf.MustLoadAndValidate(cfg,
		"simple_cli",
		structconf.WithVersion("1.0.0"),
	)

	if cfg.Greet {
		fmt.Printf("Hello %s!\n", cfg.Name)
	}
}
