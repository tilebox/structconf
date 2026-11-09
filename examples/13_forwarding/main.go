package main

import (
	"context"
	"fmt"
	"os"

	"github.com/tilebox/structconf"
	"github.com/urfave/cli/v3"
)

type RunArgs struct {
	Number  int      `default:"1" validate:"gte=1"                 help:"Number of workers" alias:"-n"`
	Target  string   `arg:"0"     help:"Execution target"          nargs:"1"`
	Command []string `arg:"1"     help:"Command and its arguments" nargs:"+"`
}

// Run from the repository root:
// go run ./examples/13_forwarding -n 2 local echo --literal-flag "a,b" ""
// This example prints the command without executing it.
func main() {
	args := &RunArgs{}
	cmd := &cli.Command{
		Name:         "run",
		Usage:        "Preview a command on an execution target",
		StopOnNthArg: new(1), // After the target, flags belong to the forwarded command.
		Action: func(_ context.Context, _ *cli.Command) error {
			fmt.Printf("Target: %s\nWorkers: %d\nCommand: %q\n", args.Target, args.Number, args.Command)
			return nil
		},
	}
	if err := structconf.BindCommand(cmd, args, structconf.WithDisableImplicitEnvVars()); err != nil {
		panic(err)
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		if _, printErr := fmt.Fprintln(os.Stderr, err); printErr != nil {
			panic(printErr)
		}
		os.Exit(1)
	}
}
