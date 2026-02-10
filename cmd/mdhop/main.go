package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var err error
	switch os.Args[1] {
	case "build":
		err = runBuild(os.Args[2:])
	case "resolve":
		err = runResolve(os.Args[2:])
	case "query":
		err = runQuery(os.Args[2:])
	case "stats":
		err = runStats(os.Args[2:])
	case "update", "add", "move", "delete", "disambiguate", "diagnose":
		err = fmt.Errorf("not yet implemented")
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `Usage: mdhop <command> [options]

Commands:
  build      Build the index from the vault
  resolve    Resolve a link from a source file
  query      Query related information for a node
  stats      Show vault statistics

Run 'mdhop <command> --help' for command-specific help.
`)
}
