package batch

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/javimcasas/smartconfigure/internal/excelsheet"
	"github.com/javimcasas/smartconfigure/internal/report"
	"github.com/javimcasas/smartconfigure/internal/sshrunner"
)

// Progress is reported once per device, right after it finishes (whether
// it succeeded or not), so both the CLI and the GUI can show live progress
// without each having to re-implement the batch loop themselves.
type Progress struct {
	Index  int // 1-based
	Total  int
	Device string
	Result report.Result
}

// Options bundles everything a batch run needs. It's shared by the CLI
// (main.go) and the GUI (internal/gui) so the two front-ends can never
// drift apart in behaviour — there is exactly one place that decides
// Run vs DryRun and writes the report.
type Options struct {
	Devices    []excelsheet.Device
	Lines      []string
	OutDir     string
	DryRun     bool
	SSH        sshrunner.Config
	OnProgress func(Progress) // optional; may be nil
}

// Run executes the template against every device in opts.Devices, writes
// per-device logs and a consolidated report.csv into opts.OutDir, and
// returns all results.
func Run(opts Options) ([]report.Result, error) {
	results := make([]report.Result, 0, len(opts.Devices))

	for i, dev := range opts.Devices {
		start := time.Now()
		logPath := filepath.Join(opts.OutDir, sanitizeFilename(dev.IP)+".log")

		var res report.Result
		if opts.DryRun {
			res = sshrunner.DryRun(dev, opts.Lines, logPath)
		} else {
			res = sshrunner.Run(dev, opts.Lines, opts.SSH, logPath)
		}
		res.Duration = time.Since(start)
		results = append(results, res)

		if opts.OnProgress != nil {
			opts.OnProgress(Progress{Index: i + 1, Total: len(opts.Devices), Device: dev.IP, Result: res})
		}
	}

	reportPath := filepath.Join(opts.OutDir, "report.csv")
	if err := report.WriteCSV(reportPath, results); err != nil {
		return results, fmt.Errorf("could not write report: %w", err)
	}
	return results, nil
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
