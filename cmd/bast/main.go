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

	switch os.Args[1] {
	case "new":
		if len(os.Args) < 3 {
			fatalf("usage: bast new <appname>")
		}
		if err := runNew(os.Args[2], "."); err != nil {
			fatalf("bast new: %v", err)
		}
		fmt.Printf("✓ Created project %s\n", os.Args[2])
		fmt.Printf("  cd %s && go mod tidy && go run .\n", os.Args[2])

	case "generate", "gen", "g":
		if len(os.Args) < 4 {
			fatalf("usage: bast generate <module|guard|service> <name>")
		}
		kind := os.Args[2]
		name := os.Args[3]
		var err error
		switch kind {
		case "module":
			err = runGenerateModule(name, ".")
		case "guard":
			err = runGenerateGuard(name, ".")
		case "service":
			err = runGenerateService(name, ".")
		default:
			fatalf("unknown generate target %q — use module, guard, or service", kind)
		}
		if err != nil {
			fatalf("bast generate %s: %v", kind, err)
		}
		fmt.Printf("✓ Generated %s/%s\n", kind, name)

	case "help", "--help", "-h":
		printUsage()

	default:
		fatalf("unknown command %q — run 'bast help'", os.Args[1])
	}
}

func printUsage() {
	fmt.Print(`Bast — production-grade HTTP framework for Go

Usage:
  bast new <appname>                  Create a new Bast project
  bast generate module <name>         Generate a module (5 files)
  bast generate guard  <name>         Generate a guard
  bast generate service <name>        Generate a shared service

Aliases: generate → gen → g
`)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "bast: "+format+"\n", args...)
	os.Exit(1)
}