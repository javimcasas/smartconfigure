package gui

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/xuri/excelize/v2"
)

// writeFixtures builds a template + a matching Excel in dir. The third
// device has an empty SUBNET_MASK so validation passes (the loader accepts
// it) but rendering must flag it.
func writeFixtures(t *testing.T, dir string, complete bool) (tmplPath, xlsxPath string) {
	t.Helper()
	tmplPath = filepath.Join(dir, "template.txt")
	if err := os.WriteFile(tmplPath, []byte("system-view\nsysname {SWITCH_NAME}\n\ninterface MEth0/0/0\n ip address {MGMT_IP} {SUBNET_MASK}\ncommit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows := [][]string{
		{"IP Equipo", "Usuario", "Contraseña", "SWITCH_NAME", "MGMT_IP", "SUBNET_MASK"},
		{"10.10.1.21", "admin", "Adm1n!", "SW-CORE-01", "10.10.1.21", "255.255.255.0"},
		{"10.10.1.22", "admin", "Adm1n!", "SW-CORE-02", "10.10.1.22", "255.255.255.0"},
		{"10.10.1.23", "admin", "Adm1n!", "SW-CORE-03", "10.10.1.23", ""},
	}
	if complete {
		rows[3][5] = "255.255.255.0"
	}
	f := excelize.NewFile()
	for r, row := range rows {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			_ = f.SetCellValue("Sheet1", cell, v)
		}
	}
	xlsxPath = filepath.Join(dir, "equipos.xlsx")
	if err := f.SaveAs(xlsxPath); err != nil {
		t.Fatal(err)
	}
	return tmplPath, xlsxPath
}

func newTestUI(t *testing.T) (*windowUI, fyne.Window) {
	t.Helper()
	a := test.NewApp()
	a.Settings().SetTheme(scTheme{a.Settings().Theme()})
	w := a.NewWindow("SmartConfigure test")
	ui := newWindowUI(w, "test")
	w.SetContent(ui.root)
	w.Resize(fyne.NewSize(860, 680))
	return ui, w
}

// waitFor polls until cond is true; the run/export handlers finish on a
// goroutine and report back through fyne.Do.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestValidateEnablesActionsAndReportsShape(t *testing.T) {
	ui, _ := newTestUI(t)
	if !ui.runBtn.Disabled() || !ui.exportBtn.Disabled() {
		t.Fatal("Run/Export must start disabled")
	}

	tmpl, xlsx := writeFixtures(t, t.TempDir(), true)
	ui.templatePath = tmpl
	ui.validate()
	if !ui.runBtn.Disabled() {
		t.Fatal("Run must stay disabled with only the template chosen")
	}

	ui.excelPath = xlsx
	ui.validate()
	if ui.loaded == nil || ui.runBtn.Disabled() || ui.exportBtn.Disabled() {
		t.Fatal("both files valid: Run/Export should be enabled")
	}
	if got := ui.summaryLabel.Text; !strings.Contains(got, "3 equipo(s)") || !strings.Contains(got, "5 línea(s)") || !strings.Contains(got, "SWITCH_NAME") {
		t.Errorf("summary should describe the batch, got %q", got)
	}
}

func TestValidateReportsLoaderErrors(t *testing.T) {
	ui, _ := newTestUI(t)
	dir := t.TempDir()
	tmpl, _ := writeFixtures(t, dir, true)
	bad := filepath.Join(dir, "bad.txt")
	_ = os.WriteFile(bad, []byte("sysname {SWITCH_NAME}\nvlan {VLAN_ID}\n"), 0o644)
	_, xlsx := writeFixtures(t, dir, true)
	_ = tmpl

	ui.templatePath, ui.excelPath = bad, xlsx
	ui.validate()
	if ui.loaded != nil || !ui.runBtn.Disabled() {
		t.Fatal("a template variable without a column must keep Run disabled")
	}
	if !strings.HasPrefix(ui.summaryLabel.Text, "✗ Excel:") || !strings.Contains(ui.summaryLabel.Text, "VLAN_ID") {
		t.Errorf("summary should name the missing column, got %q", ui.summaryLabel.Text)
	}
}

func TestDryRunAndExportThroughTheWindow(t *testing.T) {
	ui, _ := newTestUI(t)
	dir := t.TempDir()
	ui.templatePath, ui.excelPath = writeFixtures(t, dir, false)
	ui.validate()

	// Dry-run: third row has an empty SUBNET_MASK → 2 OK, 1 failed.
	ui.run()
	waitFor(t, "dry-run to finish", func() bool { return !ui.runBtn.Disabled() })
	if !strings.Contains(ui.statusLabel.Text, "2 OK, 1 con variables sin valor") {
		t.Errorf("status after dry-run: %q", ui.statusLabel.Text)
	}
	if _, err := os.Stat(filepath.Join(dir, "smartconfigure-output", "report.csv")); err != nil {
		t.Errorf("report.csv not written: %v", err)
	}

	// Export must refuse the same batch, for the same reason.
	ui.exportSecureCRT()
	waitFor(t, "export to finish", func() bool { return !ui.exportBtn.Disabled() })
	if _, err := os.Stat(filepath.Join(dir, "smartconfigure-output", "smartconfigure-securecrt.vbs")); err == nil {
		t.Error("export must not write a script when a variable is unresolved")
	}
	if !strings.Contains(strings.Join(ui.console.lines, "\n"), "{SUBNET_MASK}") {
		t.Error("console should show the unresolved placeholder")
	}

	// Fix the Excel, export again: both scripts appear.
	_, ui.excelPath = writeFixtures(t, dir, true)
	ui.validate()
	ui.exportSecureCRT()
	waitFor(t, "second export", func() bool { return !ui.exportBtn.Disabled() })
	for _, name := range []string{"smartconfigure-securecrt.vbs", "smartconfigure-securecrt.py"} {
		if _, err := os.Stat(filepath.Join(dir, "smartconfigure-output", name)); err != nil {
			t.Errorf("%s not written: %v", name, err)
		}
	}
	if !strings.Contains(ui.statusLabel.Text, "Script exportado (3 equipos)") {
		t.Errorf("status after export: %q", ui.statusLabel.Text)
	}
	if ui.openOutputBtn.Disabled() {
		t.Error("Open folder should be enabled after an export")
	}

	// Optional visual check: SC_GUI_CAPTURE=<file.png> saves what the
	// window looks like at this point (software renderer).
	if out := os.Getenv("SC_GUI_CAPTURE"); out != "" {
		img := ui.w.Canvas().Capture()
		f, err := os.Create(out)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}
}
