package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/javimcasas/smartconfigure/internal/excelsheet"
	"github.com/javimcasas/smartconfigure/internal/report"
	"github.com/javimcasas/smartconfigure/internal/sshrunner"
	"github.com/javimcasas/smartconfigure/internal/template"
)

// version is overridden at build time via -ldflags "-X main.version=..."
// by the release workflow, so `smartconfigure --version` reports the tag.
var version = "dev"

func main() {
	templatePath := flag.String("template", "", "Path to the command template file (required)")
	excelPath := flag.String("excel", "", "Path to the devices Excel file (required)")
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

	if *templatePath == "" || *excelPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: smartconfigure --template <file> --excel <file> [--out <dir>]")
		flag.PrintDefaults()
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

	cfg := sshrunner.Config{
		Port:           *port,
		ConnectTimeout: *connectTimeout,
		IdleTimeout:    *idleTimeout,
		LineTimeout:    *lineTimeout,
	}

	results := make([]report.Result, 0, len(devices))

	for i, dev := range devices {
		fmt.Printf("[%d/%d] %s ... ", i+1, len(devices), dev.IP)
		start := time.Now()

		logPath := filepath.Join(*outDir, sanitizeFilename(dev.IP)+".log")

		var res report.Result
		if *dryRun {
			res = sshrunner.DryRun(dev, tmpl.Lines, logPath)
		} else {
			res = sshrunner.Run(dev, tmpl.Lines, cfg, logPath)
		}
		res.Duration = time.Since(start)

		if res.Success {
			fmt.Printf("OK (%s)\n", res.Duration.Round(time.Millisecond))
		} else {
			fmt.Printf("FAILED (%s): %s\n", res.Duration.Round(time.Millisecond), res.Error)
		}
		results = append(results, res)
	}

	reportPath := filepath.Join(*outDir, "report.csv")
	if err := report.WriteCSV(reportPath, results); err != nil {
		fatalf("Could not write report: %v", err)
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

func sanitizeFilename(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|':
			out = append(out, '_')
		default:
			out = append(out, r)
		}
	}
	return string(out)
}
