package sshrunner

import (
	"fmt"
	"os"
	"time"

	"github.com/javimcasas/smartconfigure/internal/excelsheet"
	"github.com/javimcasas/smartconfigure/internal/report"
	"github.com/javimcasas/smartconfigure/internal/template"
)

// DryRun renders every template line for dev without opening any network
// connection, and writes the result to logPath using the same section
// format as a real run, so the two are easy to compare side by side. It
// exists to let you validate a template + Excel pairing — catching missing
// variables or typos in column names — before you have access to the
// actual equipment.
func DryRun(dev excelsheet.Device, lines []string, logPath string) report.Result {
	res := report.Result{Device: dev.IP}
	runStart := time.Now()

	logFile, ferr := os.Create(logPath)
	if ferr != nil {
		res.Error = fmt.Sprintf("could not create log file: %v", ferr)
		return res
	}
	defer logFile.Close()

	fmt.Fprintf(logFile, "SmartConfigure DRY RUN — no connection was made to this device\n")
	fmt.Fprintf(logFile, "Device:        %s\n", dev.IP)
	fmt.Fprintf(logFile, "User:          %s\n", dev.Username)
	fmt.Fprintf(logFile, "Template size: %d line(s)\n", len(lines))
	fmt.Fprintf(logFile, "Started:       %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(logFile, "%s\n\n", separator)

	res.Success = true
	for i, rawLine := range lines {
		line := template.Render(rawLine, dev.Vars)
		title := fmt.Sprintf("[%d/%d]  would send", i+1, len(lines))

		body := line
		if template.HasUnresolvedPlaceholder(line) {
			res.Success = false
			if res.Error == "" {
				res.Error = fmt.Sprintf("line %d has a variable with no value in the Excel row: %s", i+1, line)
			}
			body = line + "\n\n⚠ this line still contains an unresolved {VARIABLE} — check the matching Excel column name"
		}
		writeSection(logFile, title, body)
	}

	fmt.Fprintf(logFile, "%s\n", separator)
	if res.Success {
		fmt.Fprintf(logFile, "RESULT: DRY-RUN OK — all variables resolved, nothing was sent to the device\n")
	} else {
		fmt.Fprintf(logFile, "RESULT: DRY-RUN FAILED — %s\n", res.Error)
	}
	fmt.Fprintf(logFile, "Finished:  %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(logFile, "Duration:  %s\n", time.Since(runStart).Round(time.Millisecond))
	fmt.Fprintf(logFile, "%s\n", separator)

	return res
}
