package report

import (
	"encoding/csv"
	"os"
	"time"
)

// Result summarizes the outcome of running the template against one device.
type Result struct {
	Device   string
	Success  bool
	Error    string
	Duration time.Duration
}

// WriteCSV writes a simple report.csv with one row per device, so a batch
// run leaves behind both full per-device logs and a one-glance summary.
func WriteCSV(path string, results []Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	if err := w.Write([]string{"Device", "Status", "Duration", "Error"}); err != nil {
		return err
	}
	for _, r := range results {
		status := "OK"
		if !r.Success {
			status = "FAILED"
		}
		if err := w.Write([]string{r.Device, status, r.Duration.Round(time.Millisecond).String(), r.Error}); err != nil {
			return err
		}
	}
	return nil
}
