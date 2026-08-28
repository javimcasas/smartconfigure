package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/javimcasas/smartconfigure/internal/batch"
	"github.com/javimcasas/smartconfigure/internal/excelsheet"
	"github.com/javimcasas/smartconfigure/internal/gui"
	"github.com/javimcasas/smartconfigure/internal/sshrunner"
	"github.com/javimcasas/smartconfigure/internal/template"
)

// version is overridden at build time via -ldflags "-X main.version=..."
// by the release workflow, so `smartconfigure --version` reports the tag.
var version = "dev"

func main() {
	templatePath := flag.String("template", "", "Path to the command template file")
	excelPath := flag.String("excel", "", "Path to the devices Excel file")
	outDir := flag.String("out", "smartconfigure-output", "Output directory for logs and report")
	port := flag.Int("port", 22, "SSH port")
	connectTimeout := flag.Duration("connect-timeout", 10*time.Second, "SSH connection timeout")
	idleTimeout := flag.Duration("idle-timeout", 800*time.Millisecond, "How long to wait for the device to go quiet before sending the next line")
	lineTimeout := flag.Duration("line-timeout", 15*time.Second, "Max time to wait for a single line's output before giving up on it")
	dryRun := flag.Bool("dry-run", false, "Validate the template and Excel without connecting to any device")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("smartconfigure", version)
		return
	}

	// No --template/--excel at all: this is a double-click launch (or just
	// running the binary with no arguments), so open the graphical
	// interface instead of a bare command-line usage message. The CLI
	// flags below remain fully functional for scripted/automated use —
	// the GUI and the CLI share the exact same batch.Run() engine.
	if *templatePath == "" && *excelPath == "" {
		gui.Launch(version)
		return
	}

	if *templatePath == "" || *excelPath == "" {
		fmt.Fprintln(os.Stderr, "Both --template and --excel are required together on the command line.")
		fmt.Fprintln(os.Stderr, "Run smartconfigure with no flags at all to use the graphical interface instead.")
		os.Exit(1)
	}

	tmpl, err := template.Load(*templatePath)
	if err != nil {
		fatalf("Could not read template: %v", err)
	}

	devices, err := excelsheet.Load(*excelPath, tmpl.Variables)
	if err != nil {
		fatalf("Could not read Excel file: %v", err)
	}
	if len(devices) == 0 {
		fatalf("The Excel file has no device rows to process.")
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fatalf("Could not create output directory: %v", err)
	}

	if *dryRun {
		fmt.Println("Running in DRY-RUN mode — no device will be contacted.")
	}
	fmt.Printf("Loaded template with %d line(s) and %d variable(s): %v\n", len(tmpl.Lines), len(tmpl.Variables), tmpl.Variables)
	fmt.Printf("Loaded %d device(s) from %s\n\n", len(devices), *excelPath)

	results, err := batch.Run(batch.Options{
		Devices: devices,
		Lines:   tmpl.Lines,
		OutDir:  *outDir,
		DryRun:  *dryRun,
		SSH: sshrunner.Config{
			Port:           *port,
			ConnectTimeout: *connectTimeout,
			IdleTimeout:    *idleTimeout,
			LineTimeout:    *lineTimeout,
		},
		OnProgress: func(p batch.Progress) {
			if p.Result.Success {
				fmt.Printf("[%d/%d] %s ... OK (%s)\n", p.Index, p.Total, p.Device, p.Result.Duration.Round(time.Millisecond))
			} else {
				fmt.Printf("[%d/%d] %s ... FAILED (%s): %s\n", p.Index, p.Total, p.Device, p.Result.Duration.Round(time.Millisecond), p.Result.Error)
			}
		},
	})
	if err != nil {
		fatalf("%v", err)
	}

	ok := 0
	for _, r := range results {
		if r.Success {
			ok++
		}
	}
	fmt.Printf("\nDone: %d/%d succeeded. Logs and report saved in %s\n", ok, len(results), *outDir)

	if ok != len(results) {
		os.Exit(1)
	}
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
