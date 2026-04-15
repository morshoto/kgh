package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/shotomorisk/kgh/internal/execution"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	return runWithIO(args, os.Stdout, os.Stderr)
}

func runWithIO(args []string, stdout, stderr io.Writer) int {
	args = stripCommandName(args)

	if len(args) == 0 {
		printUsage(stdout)
		return 0
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, version)
		return 0
	case "help", "--help", "-h":
		printUsage(stdout)
		return 0
	case "run":
		code, err := runCommand(context.Background(), args[1:], stdout, stderr)
		if err != nil {
			fmt.Fprintln(stderr, err)
		}
		return code
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printUsage(stderr)
		return 1
	}
}

type runFlags struct {
	target       string
	dryRun       bool
	gpu          *bool
	internet     *bool
	pollInterval time.Duration
	timeout      time.Duration
}

func runCommand(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	flags, err := parseRunFlags(args)
	if err != nil {
		return 1, err
	}

	runner := execution.NewRunner(nil)
	report, err := runner.Execute(ctx, execution.Request{
		Target:       flags.target,
		DryRun:       flags.dryRun,
		GPU:          flags.gpu,
		Internet:     flags.internet,
		ConfigPath:   execution.DefaultConfigPath,
		PollInterval: flags.pollInterval,
		PollTimeout:  flags.timeout,
	})
	if err != nil {
		return 1, err
	}

	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return 1, fmt.Errorf("marshal run output: %w", err)
	}

	if _, err := stdout.Write(append(payload, '\n')); err != nil {
		return 1, err
	}
	return 0, nil
}

func parseRunFlags(args []string) (runFlags, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var flags runFlags

	fs.StringVar(&flags.target, "target", "", "target name to run")
	fs.BoolVar(&flags.dryRun, "dry-run", true, "print resolved execution JSON without invoking Kaggle")
	fs.DurationVar(&flags.pollInterval, "poll-interval", 0, "poll interval for live Kaggle status checks")
	fs.DurationVar(&flags.timeout, "timeout", 0, "timeout for live Kaggle polling")
	fs.Func("gpu", "override GPU setting with true or false", func(value string) error {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid value for --gpu: %q: expected true or false", value)
		}
		flags.gpu = boolPtr(parsed)
		return nil
	})
	fs.Func("internet", "override internet setting with true or false", func(value string) error {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid value for --internet: %q: expected true or false", value)
		}
		flags.internet = boolPtr(parsed)
		return nil
	})

	if err := fs.Parse(args); err != nil {
		return runFlags{}, err
	}
	if len(fs.Args()) != 0 {
		return runFlags{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if flags.pollInterval < 0 {
		return runFlags{}, fmt.Errorf("--poll-interval must be greater than or equal to 0")
	}
	if flags.timeout < 0 {
		return runFlags{}, fmt.Errorf("--timeout must be greater than or equal to 0")
	}
	if strings.TrimSpace(flags.target) == "" {
		return runFlags{}, fmt.Errorf("--target is required")
	}
	return flags, nil
}

func boolPtr(v bool) *bool {
	return &v
}

func printUsage(w io.Writer) {
	fmt.Fprint(w, `kgh is a GitHub-native CLI for Kaggle workflows.

Usage:
  kgh <command>

Available Commands:
  version   Print the current kgh version
  help      Show this help message
  run       Resolve and execute a Kaggle target

Examples:
  kgh run --target exp142
  kgh run --target exp142 --poll-interval=2s --timeout=5m
  kgh run --target exp142 --dry-run=false
  kgh version
`)
}

func stripCommandName(args []string) []string {
	if len(args) == 0 {
		return args
	}

	switch args[0] {
	case "kgh", "kh":
		return args[1:]
	default:
		return args
	}
}
