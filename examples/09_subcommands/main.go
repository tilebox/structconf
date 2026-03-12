package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tilebox/structconf"
	"github.com/urfave/cli/v3"
)

type GreetConfig struct {
	Name string `default:"World" help:"Whom to greet"`
	Loud bool   `help:"Print in uppercase"`
}

type SumConfig struct {
	Left  int `default:"1" help:"Left operand"`
	Right int `default:"2" help:"Right operand"`
}

// usage:
// ./subcommands greet --name Tilebox --loud
// ./subcommands sum --left 10 --right 5
func main() {
	appName := filepath.Base(os.Args[0])
	if appName == "" {
		appName = "subcommands"
	}

	greetCfg := &GreetConfig{}
	greetCmd, err := structconf.NewCommand(greetCfg, "greet", func(ctx context.Context, cmd *cli.Command) error {
		if greetCfg.Loud {
			fmt.Println(strings.ToUpper(greetCfg.Name))
			return nil
		}

		fmt.Println(greetCfg.Name)
		return nil
	}, structconf.WithDescription("Print a greeting"))
	if err != nil {
		panic(err)
	}

	sumCfg := &SumConfig{}
	sumCmd := &cli.Command{
		Name:  "sum",
		Usage: "Add two integers",
		Action: func(ctx context.Context, cmd *cli.Command) error {
			fmt.Println(sumCfg.Left + sumCfg.Right)
			return nil
		},
	}

	err = structconf.BindCommand(sumCmd, sumCfg)
	if err != nil {
		panic(err)
	}

	root := &cli.Command{
		Name:                  appName,
		Usage:                 "Example app showing structconf subcommands",
		EnableShellCompletion: true,
		Commands:              []*cli.Command{greetCmd, sumCmd},
	}

	if err := root.Run(context.Background(), os.Args); err != nil {
		panic(err)
	}
}
