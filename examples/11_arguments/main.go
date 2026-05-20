package main

import (
	"fmt"
	"strings"

	"github.com/tilebox/structconf"
)

type Config struct {
	Greeting string `flag:"greeting" default:"Hello"           help:"Greeting to print"`
	Name     string `arg:"0"         env:"NAME"                default:"World"                              help:"Name to greet"`
	Count    int    `arg:"1"         default:"1"               help:"Number of times to print the greeting"`
	Loud     bool   `flag:"loud"     help:"Print in uppercase"`
}

// usage:
// ./arguments Tilebox 3 --greeting Hi --loud
func main() {
	cfg := &Config{}
	structconf.MustLoad(cfg, "arguments")

	message := fmt.Sprintf("%s, %s!", cfg.Greeting, cfg.Name)
	if cfg.Loud {
		message = strings.ToUpper(message)
	}

	for range cfg.Count {
		fmt.Println(message)
	}
}
