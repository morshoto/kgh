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
	ghctx "github.com/shotomorisk/kgh/internal/github"
	"github.com/shotomorisk/kgh/internal/parser"
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
	case "github":
		code, err := githubCommand(context.Background(), args[1:], stdout, stderr)
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

type sharedRunFlags struct {
	dryRun       bool
	pollInterval time.Duration
	timeout      time.Duration
}

type runFlags struct {
	target   string
	gpu      *bool
	internet *bool
	sharedRunFlags
}

type githubRunFlags struct {
	sharedRunFlags
}

func runCommand(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	flags, err := parseRunFlags(args)
	if err != nil {
		return 1, err
	}

	return executeRequest(ctx, execution.Request{
		Target:       flags.target,
		DryRun:       flags.dryRun,
		GPU:          flags.gpu,
		Internet:     flags.internet,
		ConfigPath:   execution.DefaultConfigPath,
		PollInterval: flags.pollInterval,
		PollTimeout:  flags.timeout,
	}, stdout)
}

func githubCommand(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	if len(args) == 0 {
		printGitHubUsage(stdout)
		return 0, nil
	}

	switch args[0] {
	case "help", "--help", "-h":
		printGitHubUsage(stdout)
		return 0, nil
	case "run":
	default:
		return 1, fmt.Errorf("unknown github subcommand: %s", args[0])
	}

	flags, err := parseGitHubRunFlags(args[1:])
	if err != nil {
		return 1, err
	}

	trigger, err := ghctx.NewTriggerResolver().Resolve(ctx)
	if err != nil {
		return 1, err
	}

	return executeRequest(ctx, requestFromTrigger(trigger, flags.sharedRunFlags), stdout)
}

func executeRequest(ctx context.Context, req execution.Request, stdout io.Writer) (int, error) {
	runner := execution.NewRunner(nil)
	report, err := runner.Execute(ctx, req)
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
	if err := validateSharedRunFlags(flags.sharedRunFlags); err != nil {
		return runFlags{}, err
	}
	if strings.TrimSpace(flags.target) == "" {
		return runFlags{}, fmt.Errorf("--target is required")
	}
	return flags, nil
}

func parseGitHubRunFlags(args []string) (githubRunFlags, error) {
	fs := flag.NewFlagSet("github run", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var flags githubRunFlags

	fs.BoolVar(&flags.dryRun, "dry-run", true, "print resolved execution JSON without invoking Kaggle")
	fs.DurationVar(&flags.pollInterval, "poll-interval", 0, "poll interval for live Kaggle status checks")
	fs.DurationVar(&flags.timeout, "timeout", 0, "timeout for live Kaggle polling")

	if err := fs.Parse(args); err != nil {
		return githubRunFlags{}, err
	}
	if len(fs.Args()) != 0 {
		return githubRunFlags{}, fmt.Errorf("unexpected arguments: %s", strings.Join(fs.Args(), " "))
	}
	if err := validateSharedRunFlags(flags.sharedRunFlags); err != nil {
		return githubRunFlags{}, err
	}
	return flags, nil
}

func validateSharedRunFlags(flags sharedRunFlags) error {
	if flags.pollInterval < 0 {
		return fmt.Errorf("--poll-interval must be greater than or equal to 0")
	}
	if flags.timeout < 0 {
		return fmt.Errorf("--timeout must be greater than or equal to 0")
	}
	return nil
}

func requestFromTrigger(trigger parser.Trigger, flags sharedRunFlags) execution.Request {
	return execution.Request{
		Target:       trigger.Target,
		DryRun:       flags.dryRun,
		GPU:          trigger.GPU,
		Internet:     trigger.Internet,
		ConfigPath:   execution.DefaultConfigPath,
		PollInterval: flags.pollInterval,
		PollTimeout:  flags.timeout,
	}
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
  github    Resolve and execute from GitHub commit metadata
  run       Resolve and execute a Kaggle target

Examples:
  kgh run --target exp142
  kgh run --target exp142 --poll-interval=2s --timeout=5m
  kgh run --target exp142 --dry-run=false
  kgh github run
  kgh version
`)
}

func printGitHubUsage(w io.Writer) {
	fmt.Fprint(w, `kgh github resolves trigger intent from GitHub commit metadata.

Usage:
  kgh github run [flags]

Flags:
  --dry-run         print resolved execution JSON without invoking Kaggle (default true)
  --poll-interval   poll interval for live Kaggle status checks
  --timeout         timeout for live Kaggle polling

Environment:
  GITHUB_EVENT_NAME   GitHub Actions event name
  GITHUB_EVENT_PATH   path to the GitHub event payload JSON
  GITHUB_SHA          current commit SHA for push events
  GITHUB_WORKSPACE    repository checkout path for reading commit metadata
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
