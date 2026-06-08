package main

import (
	"fmt"
	"os"
	"path/filepath"
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
		var (
			err    error
			outDir string
		)
		switch kind {
		case "module":
			outDir = filepath.Join("modules", name)
			err = runGenerateModule(name, outDir)
			if err == nil {
				if injectErr := injectModuleIntoMain(name); injectErr != nil {
					fmt.Printf("  ⚠ could not auto-register in main.go: %v\n", injectErr)
					fmt.Printf("  add manually: %s.NewModule() inside app.Register(...)\n", toPackageName(name))
				} else {
					fmt.Printf("  ↳ registered in main.go\n")
				}
			}
		case "guard":
			outDir = filepath.Join("shared", "guards")
			err = runGenerateGuard(name, outDir)
		case "service":
			outDir = filepath.Join("shared", "services")
			err = runGenerateService(name, outDir)
		default:
			fatalf("unknown generate target %q — use module, guard, or service", kind)
		}
		if err != nil {
			fatalf("bast generate %s: %v", kind, err)
		}
		fmt.Printf("✓ Generated %s/%s → %s\n", kind, name, outDir)

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