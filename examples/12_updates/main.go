package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tilebox/structconf"
	"github.com/urfave/cli/v3"
)

type UpdateArgs struct {
	Workflow    string  `arg:"0"                                       validate:"required" help:"Workflow to update" nargs:"1"`
	Summary     *string `help:"New summary; an empty string clears it"`
	SummaryFile *string `help:"Read the summary from a file"           file:"true"`
	Enabled     *bool   `help:"Enable or disable the workflow"`
}

// Run from the repository root:
// go run ./examples/12_updates update --summary "" --enabled=false my-workflow
// go run ./examples/12_updates update --summary-file README.md my-workflow
// This example prints the requested changes without updating a remote service.
func main() {
	args := &UpdateArgs{}
	cmd := &cli.Command{
		Name:  "update",
		Usage: "Preview a workflow update",
		Action: func(_ context.Context, _ *cli.Command) error {
			if args.SummaryFile != nil {
				// file:"true" only enables completion; the action owns file I/O.
				data, err := os.ReadFile(*args.SummaryFile)
				if err != nil {
					return fmt.Errorf("read summary: %w", err)
				}
				summary := string(data)
				args.Summary = &summary
			}
			if args.Summary != nil {
				fmt.Printf("Set %s summary to %q\n", args.Workflow, *args.Summary)
			}
			if args.Enabled != nil {
				fmt.Printf("Set %s enabled to %t\n", args.Workflow, *args.Enabled)
			}
			return nil
		},
	}
	if err := structconf.BindCommand(cmd, args,
		structconf.WithDisableImplicitEnvVars(),
		structconf.WithValidator(validateUpdate),
	); err != nil {
		panic(err)
	}
	root := &cli.Command{
		Name:                  "workflows",
		EnableShellCompletion: true,
		Commands:              []*cli.Command{cmd},
	}
	if err := root.Run(context.Background(), os.Args); err != nil {
		if _, printErr := fmt.Fprintln(os.Stderr, renderError(err)); printErr != nil {
			panic(printErr)
		}
		os.Exit(1)
	}
}

func validateUpdate(_ *cli.Command, config any) error {
	if err := structconf.Validate(config); err != nil {
		return err
	}
	args := config.(*UpdateArgs)
	if args.Summary != nil && args.SummaryFile != nil {
		return structconf.NewValidationError(
			"choose either --summary or --summary-file", args, "Summary", "SummaryFile",
		)
	}
	if args.Summary == nil && args.SummaryFile == nil && args.Enabled == nil {
		return structconf.NewValidationError(
			"provide at least one update", args, "Summary", "SummaryFile", "Enabled",
		)
	}
	return nil
}

func renderError(err error) string {
	var validation *structconf.ValidationError
	if !errors.As(err, &validation) {
		return err.Error()
	}
	var lines []string
	for _, issue := range validation.Issues {
		lines = append(lines, issue.Message)
		for _, field := range issue.Fields {
			if field.Name != "" {
				lines = append(lines, fmt.Sprintf("  %s: %s", field.Name, field.Help))
			}
		}
	}
	return strings.Join(lines, "\n")
}
