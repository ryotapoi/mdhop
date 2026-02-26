package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"
)

var version = "dev"

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
	case "diagnose":
		err = runDiagnose(os.Args[2:])
	case "delete":
		err = runDelete(os.Args[2:])
	case "update":
		err = runUpdate(os.Args[2:])
	case "add":
		err = runAdd(os.Args[2:])
	case "move":
		err = runMove(os.Args[2:])
	case "disambiguate":
		err = runDisambiguate(os.Args[2:])
	case "simplify":
		err = runSimplify(os.Args[2:])
	case "repair":
		err = runRepair(os.Args[2:])
	case "convert":
		err = runConvert(os.Args[2:])
	case "--version":
		printVersion(os.Stdout)
		return
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printVersion(w io.Writer) {
	v := version
	if v == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
	}
	fmt.Fprintf(w, "mdhop version %s\n", v)
}

func printUsage() {
	fmt.Fprint(os.Stderr, `Usage: mdhop <command> [options]

Index Commands:
  build         Build the index from the vault
  add           Add new files to the index
  update        Update specified files in the index
  delete        Remove files from the index
  move          Move a file and update links
  disambiguate  Rewrite basename links to full paths
  simplify      Shorten path links to basename when unambiguous
  repair        Fix broken path links by rewriting to basename
  convert       Convert between wikilink and markdown link formats

Query Commands:
  resolve    Resolve a link from a source file
  query      Query related information for a node
  stats      Show vault statistics
  diagnose   Show basename conflicts and phantom nodes

Run 'mdhop <command> --help' for command-specific help.
Use 'mdhop --version' for version information.
`)
}
