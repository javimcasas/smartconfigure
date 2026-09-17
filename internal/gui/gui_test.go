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

const fixtureTemplate = "system-view\nsysname {SWITCH_NAME}\n\ninterface MEth0/0/0\n ip address {MGMT_IP} {SUBNET_MASK}\ncommit\n"

func writeTemplate(t *testing.T, dir string) string {
	t.Helper()
	p := filepath.Join(dir, "template.txt")
	if err := os.WriteFile(p, []byte(fixtureTemplate), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// writeDevices builds a filled Excel at path for the fixture template. The
// third device has an empty SUBNET_MASK unless complete is set, so the
// loader accepts it but rendering must flag it.
func writeDevices(t *testing.T, path string, complete bool) {
	t.Helper()
	rows := [][]string{
		{"IP Address", "Username", "Password", "SWITCH_NAME", "MGMT_IP", "SUBNET_MASK"},
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
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
}

func newTestUI(t *testing.T) *windowUI {
	t.Helper()
	a := test.NewApp()
	a.Settings().SetTheme(scTheme{a.Settings().Theme()})
	w := a.NewWindow("SmartConfigure test")
	ui := newWindowUI(w, "test")
	w.SetContent(ui.root)
	w.Resize(fyne.NewSize(880, 780))
	return ui
}

// chooseTemplate mirrors what the file dialog callback does.
func (ui *windowUI) chooseTemplate(p string) {
	ui.templatePath = p
	ui.templateLabel.SetText(displayPath(p))
	ui.loadTemplate()
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

func TestTemplateGatesEverything(t *testing.T) {
	ui := newTestUI(t)
	if !ui.generateBtn.Disabled() || !ui.runBtn.Disabled() || !ui.exportBtn.Disabled() {
		t.Fatal("Generate/Run/Export must start disabled")
	}

	dir := t.TempDir()
	ui.chooseTemplate(writeTemplate(t, dir))
	if ui.tmpl == nil || ui.generateBtn.Disabled() {
		t.Fatal("a valid template must enable Generate Excel")
	}
	if got := ui.templateSummary.Text; !strings.Contains(got, "5 lines") || !strings.Contains(got, "3 variables") || !strings.Contains(got, "SUBNET_MASK") {
		t.Errorf("template summary should describe the template, got %q", got)
	}
	if !ui.runBtn.Disabled() {
		t.Fatal("Run must stay disabled until an Excel is chosen")
	}

	ui.chooseTemplate(filepath.Join(dir, "missing.txt"))
	if ui.tmpl != nil || !ui.generateBtn.Disabled() || !strings.HasPrefix(ui.templateSummary.Text, "✗ Template:") {
		t.Errorf("a broken template must disable Generate and report the error, got %q", ui.templateSummary.Text)
	}
}

func TestGenerateExcelThenFillThenRun(t *testing.T) {
	ui := newTestUI(t)
	dir := t.TempDir()
	ui.chooseTemplate(writeTemplate(t, dir))

	// Generate: the Excel appears next to the template and becomes the
	// chosen one. Run is enabled even though the sheet is still empty.
	ui.generateExcel()
	want := filepath.Join(dir, "devices.xlsx")
	if ui.excelPath != want {
		t.Fatalf("excelPath = %q, want %q", ui.excelPath, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("generated Excel missing: %v", err)
	}
	if !strings.Contains(ui.devicesSummary.Text, "devices.xlsx created") {
		t.Errorf("devices summary should explain the next step, got %q", ui.devicesSummary.Text)
	}
	if ui.runBtn.Disabled() || ui.exportBtn.Disabled() || ui.openExcelBtn.Disabled() {
		t.Fatal("Run/Export/Open in Excel must be enabled once an Excel exists")
	}

	// Generating again must not clobber the first file.
	ui.generateExcel()
	if ui.excelPath != filepath.Join(dir, "devices-2.xlsx") {
		t.Fatalf("second generate should suffix the name, got %q", ui.excelPath)
	}
	ui.setExcel(want)

	// Running with the sheet still empty is refused, in English, without
	// touching the console.
	ui.run()
	if !strings.HasPrefix(ui.devicesSummary.Text, "✗ Excel:") || !strings.Contains(ui.statusLabel.Text, "Nothing was run") {
		t.Errorf("empty sheet: summary=%q status=%q", ui.devicesSummary.Text, ui.statusLabel.Text)
	}

	// The user fills the sheet outside the app; Run re-reads it from disk.
	writeDevices(t, want, true)
	ui.run()
	waitFor(t, "dry-run to finish", func() bool { return !ui.runBtn.Disabled() })
	if !strings.Contains(ui.statusLabel.Text, "Dry-run done: 3 OK, 0 with missing values.") {
		t.Errorf("status after dry-run: %q", ui.statusLabel.Text)
	}
	if !strings.Contains(ui.devicesSummary.Text, "✓ 3 devices") {
		t.Errorf("devices summary after run: %q", ui.devicesSummary.Text)
	}
	if _, err := os.Stat(filepath.Join(dir, "smartconfigure-output", "report.csv")); err != nil {
		t.Errorf("report.csv not written: %v", err)
	}
}

func TestChooseExcelReportsLoaderErrors(t *testing.T) {
	ui := newTestUI(t)
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.txt")
	_ = os.WriteFile(bad, []byte("sysname {SWITCH_NAME}\nvlan {VLAN_ID}\n"), 0o644)
	xlsx := filepath.Join(dir, "devices.xlsx")
	writeDevices(t, xlsx, true)

	ui.chooseTemplate(bad)
	ui.setExcel(xlsx)
	ui.checkDevices()
	if !strings.HasPrefix(ui.devicesSummary.Text, "✗ Excel:") || !strings.Contains(ui.devicesSummary.Text, "VLAN_ID") {
		t.Errorf("summary should name the missing column, got %q", ui.devicesSummary.Text)
	}
	// Run stays enabled: the user is expected to fix the sheet and retry,
	// and the action re-reads it.
	if ui.runBtn.Disabled() {
		t.Error("Run should stay enabled so the fixed sheet can be retried")
	}
}

func TestDryRunAndExportThroughTheWindow(t *testing.T) {
	ui := newTestUI(t)
	dir := t.TempDir()
	ui.chooseTemplate(writeTemplate(t, dir))
	xlsx := filepath.Join(dir, "devices.xlsx")
	writeDevices(t, xlsx, false)
	ui.setExcel(xlsx)
	ui.checkDevices()
	if ui.devicesSummary.Text != "✓ 3 devices" {
		t.Fatalf("devices summary: %q", ui.devicesSummary.Text)
	}

	// Dry-run: third row has an empty SUBNET_MASK → 2 OK, 1 failed.
	ui.run()
	waitFor(t, "dry-run to finish", func() bool { return !ui.runBtn.Disabled() })
	if !strings.Contains(ui.statusLabel.Text, "2 OK, 1 with missing values") {
		t.Errorf("status after dry-run: %q", ui.statusLabel.Text)
	}
	console := strings.Join(ui.console.lines, "\n")
	if !strings.Contains(console, "Mode: DRY-RUN") || !strings.Contains(console, "... FAILED (") {
		t.Errorf("console should show the mode and the failed device, got:\n%s", console)
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

	// Fix the Excel in place (no re-pick), export again: both scripts appear.
	writeDevices(t, xlsx, true)
	ui.exportSecureCRT()
	waitFor(t, "second export", func() bool { return !ui.exportBtn.Disabled() })
	for _, name := range []string{"smartconfigure-securecrt.vbs", "smartconfigure-securecrt.py"} {
		if _, err := os.Stat(filepath.Join(dir, "smartconfigure-output", name)); err != nil {
			t.Errorf("%s not written: %v", name, err)
		}
	}
	if !strings.Contains(ui.statusLabel.Text, "Script exported (3 devices)") {
		t.Errorf("status after export: %q", ui.statusLabel.Text)
	}
	if ui.openOutBtn.Disabled() {
		t.Error("Open output folder should be enabled after an export")
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
