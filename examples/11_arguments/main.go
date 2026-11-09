package main

import (
	"fmt"
	"strings"

	"github.com/tilebox/structconf"
)

type Config struct {
	Greeting string `flag:"greeting" default:"Hello"           help:"Greeting to print"`
	Name     string `arg:"0"         help:"Name to greet"      nargs:"1"`
	Count    int    `arg:"1"         default:"1"               validate:"gte=1"         help:"Number of times to print the greeting" nargs:"?"`
	Loud     bool   `flag:"loud"     help:"Print in uppercase"`
}

// Run from the repository root:
// go run ./examples/11_arguments --greeting Hi --loud Tilebox 3
// go run ./examples/11_arguments Tilebox
func main() {
	cfg := &Config{}
	structconf.MustLoad(cfg, "arguments", structconf.WithDisableImplicitEnvVars())

	message := fmt.Sprintf("%s, %s!", cfg.Greeting, cfg.Name)
	if cfg.Loud {
		message = strings.ToUpper(message)
	}

	for range cfg.Count {
		fmt.Println(message)
	}
}
