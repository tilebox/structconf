package main

import (
	"fmt"

	"github.com/tilebox/structconf"
)

type ProgramConfig struct {
	Name  string `default:"World"                help:"Whom to greet"`
	Greet bool   `help:"Whether or not to greet"`
}

// usage: ./simple_cli --greet
// or check the generated help message
// ./simple_cli -h
func main() {
	cfg := &ProgramConfig{}
	structconf.MustLoadAndValidate(cfg,
		"greetings",
		structconf.WithVersion("1.0.0"),
		structconf.WithDescription("Print a greeting"),
		structconf.WithLongDescription("A CLI for printing a greeting to the console"),
	)

	if cfg.Greet {
		fmt.Printf("Hello %s!\n", cfg.Name)
	}
}
