package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	args = stripCommandName(args)

	if len(args) == 0 {
		printUsage()
		return 0
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println(version)
		return 0
	case "help", "--help", "-h":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		printUsage()
		return 1
	}
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

func printUsage() {
	fmt.Print(`kgh is a GitHub-native CLI for Kaggle workflows.

Usage:
  kgh <command>

Available Commands:
  version   Print the current kgh version
  help      Show this help message

Examples:
  kgh version
`)
}
