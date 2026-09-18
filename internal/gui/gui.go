package gui

import (
	_ "embed"
	"fmt"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/javimcasas/smartconfigure/internal/batch"
	"github.com/javimcasas/smartconfigure/internal/excelsheet"
	"github.com/javimcasas/smartconfigure/internal/securecrt"
	"github.com/javimcasas/smartconfigure/internal/sshrunner"
	"github.com/javimcasas/smartconfigure/internal/template"
)

// icon.png is the 512px brand mark (source: assets/icon.svg). A PNG, not
// the SVG: Fyne rasterises SVG icons at whatever size the OS asks for and
// the thin web glyph turned to mush in a 24px taskbar slot.
//
//go:embed icon.png
var iconPNG []byte

// defaultSSH is what the window uses for a live run and for the SecureCRT
// export, so a script produced here behaves like a direct run. The CLI
// exposes the same values as flags.
var defaultSSH = sshrunner.Config{
	Port:           22,
	ConnectTimeout: 10 * time.Second,
	IdleTimeout:    800 * time.Millisecond,
	LineTimeout:    15 * time.Second,
}

// ─── Theme ─────────────────────────────────────────────────────────────────

// scTheme is Fyne's default theme with the ecosystem's blue as the primary
// colour (design-system/smartconfigure/MASTER.md §2), so the Run button
// matches the landing page. Everything else is stock Fyne.
type scTheme struct{ fyne.Theme }

func (t scTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNamePrimary:
		if variant == theme.VariantDark {
			return color.NRGBA{R: 0x60, G: 0xA5, B: 0xFA, A: 0xFF}
		}
		return color.NRGBA{R: 0x1D, G: 0x4E, B: 0xD8, A: 0xFF}
	case theme.ColorNameForegroundOnPrimary:
		if variant == theme.VariantDark {
			return color.NRGBA{R: 0x0B, G: 0x1A, B: 0x33, A: 0xFF}
		}
		return color.White
	}
	return t.Theme.Color(name, variant)
}

// ─── Window ────────────────────────────────────────────────────────────────

// Launch opens the SmartConfigure graphical interface. It's the default
// entry point when the binary is run with no flags (i.e. double-clicked
// from Explorer/Finder), so most users never need to touch a terminal.
// It shares the exact same batch.Run() engine, Excel generator and
// SecureCRT generator as the CLI path in main.go, so behaviour never
// drifts between the two.
func Launch(version string) {
	a := app.New()
	a.Settings().SetTheme(scTheme{theme.DefaultTheme()})
	icon := fyne.NewStaticResource("smartconfigure.png", iconPNG)
	a.SetIcon(icon)

	w := a.NewWindow("SmartConfigure " + version)
	w.SetIcon(icon)
	w.Resize(fyne.NewSize(880, 720))
	w.CenterOnScreen()

	ui := newWindowUI(w, version)
	w.SetContent(ui.root)
	keepLogicalSizeAcrossMonitors(w)
	w.ShowAndRun()
}

// keepLogicalSizeAcrossMonitors works around a Fyne bug (2.6–2.8.1) on
// Windows setups whose monitors have different DPI: when the window
// crosses to a monitor with another scale, Fyne re-scales the content but
// does not reliably re-size the native window, so every 150% → 100% → 150%
// round trip shrinks it by the DPI ratio (and pointer mapping goes with
// it). The logical size is the invariant, so remember it while the scale
// is stable and re-apply it through Resize the moment the scale changes.
func keepLogicalSizeAcrossMonitors(w fyne.Window) {
	done := make(chan struct{})
	w.SetOnClosed(func() { close(done) })

	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		var lastScale float32
		var lastSize fyne.Size
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
			}
			fyne.DoAndWait(func() {
				c := w.Canvas()
				scale, size := c.Scale(), c.Size()
				switch {
				case lastScale == 0:
					lastScale, lastSize = scale, size
				case scale != lastScale:
					lastScale = scale
					w.Resize(lastSize)
				case absDiff(size.Width, lastSize.Width) > 1 || absDiff(size.Height, lastSize.Height) > 1:
					// A real resize by the user; sub-unit wobble is just
					// pixel rounding at the current scale and must not
					// accumulate across hops.
					lastSize = size
				}
			})
		}
	}()
}

// windowUI holds every widget the handlers need. State lives here, not in
// closures, so the actions (load template, generate Excel, check devices,
// run, export) share it.
//
// The workflow is: choose a template → generate its Excel (or pick one) →
// fill the Excel outside the app → Run / Export. Because the Excel is
// edited *after* being chosen, Run and Export always re-read it from disk.
type windowUI struct {
	w       fyne.Window
	version string

	templatePath string
	tmpl         *template.Template // nil until the template parses
	excelPath    string
	lastOutDir   string

	templateLabel, templateSummary *widget.Label
	generateBtn                    *widget.Button
	excelLabel, devicesSummary     *widget.Label
	openExcelBtn                   *widget.Button
	dryRunCheck                    *widget.Check
	runBtn, exportBtn, openOutBtn  *widget.Button
	progress                       *widget.ProgressBar
	statusLabel                    *widget.Label
	console                        *consoleView

	root *fyne.Container
}

func newWindowUI(w fyne.Window, version string) *windowUI {
	ui := &windowUI{w: w, version: version}

	// ── Header ── brand mark + name at 20px, like the web topbar.
	mark := canvas.NewImageFromResource(fyne.NewStaticResource("smartconfigure.png", iconPNG))
	mark.FillMode = canvas.ImageFillContain
	mark.SetMinSize(fyne.NewSize(28, 28))
	title := canvas.NewText("SmartConfigure", theme.Color(theme.ColorNameForeground))
	title.TextSize = 20
	title.TextStyle = fyne.TextStyle{Bold: true}
	subtitle := widget.NewLabel("Push one command template to every device listed in an Excel sheet, over SSH.")
	subtitle.Wrapping = fyne.TextWrapWord
	header := container.NewVBox(container.NewHBox(mark, container.NewCenter(title)), subtitle)

	// ── 1. Template ──
	ui.templateLabel = pathLabel("No template selected")
	ui.templateSummary = wrappedLabel("")
	pickTemplateBtn := widget.NewButtonWithIcon("Choose template (.txt)…", theme.DocumentIcon(), func() {
		ui.pickFile([]string{".txt"}, func(p string) {
			ui.templatePath = p
			ui.templateLabel.SetText(displayPath(p))
			ui.loadTemplate()
		})
	})
	ui.generateBtn = widget.NewButtonWithIcon("Generate Excel", theme.ContentAddIcon(), ui.generateExcel)
	ui.generateBtn.Importance = widget.MediumImportance
	ui.generateBtn.Disable()
	templateSection := section("1. Template",
		container.New(newFormLayout(), pickTemplateBtn, ui.templateLabel),
		ui.templateSummary,
		container.NewHBox(ui.generateBtn),
	)

	// ── 2. Devices ──
	ui.excelLabel = pathLabel("No Excel selected — generate one from the template, or choose an existing file")
	ui.devicesSummary = wrappedLabel("")
	pickExcelBtn := widget.NewButtonWithIcon("Choose Excel (.xlsx)…", theme.ListIcon(), func() {
		ui.pickFile([]string{".xlsx"}, func(p string) {
			ui.setExcel(p)
			ui.checkDevices()
		})
	})
	ui.openExcelBtn = widget.NewButtonWithIcon("Open in Excel", theme.FileIcon(), func() {
		if ui.excelPath != "" {
			openPath(ui.excelPath)
		}
	})
	ui.openExcelBtn.Importance = widget.LowImportance
	ui.openExcelBtn.Disable()
	devicesSection := section("2. Devices",
		container.New(newFormLayout(), pickExcelBtn, ui.excelLabel),
		ui.devicesSummary,
		container.NewHBox(ui.openExcelBtn),
	)

	// ── 3. Run ──
	ui.dryRunCheck = widget.NewCheck("Dry-run (validate only — nothing is sent to any device)", nil)
	ui.dryRunCheck.SetChecked(true)

	ui.runBtn = widget.NewButtonWithIcon("Run", theme.MediaPlayIcon(), ui.run)
	ui.runBtn.Importance = widget.HighImportance
	ui.runBtn.Disable()

	ui.exportBtn = widget.NewButtonWithIcon("Export SecureCRT script", theme.DownloadIcon(), ui.exportSecureCRT)
	ui.exportBtn.Importance = widget.MediumImportance
	ui.exportBtn.Disable()

	ui.openOutBtn = widget.NewButtonWithIcon("Open output folder", theme.FolderOpenIcon(), func() {
		if ui.lastOutDir != "" {
			openPath(ui.lastOutDir)
		}
	})
	ui.openOutBtn.Importance = widget.LowImportance
	ui.openOutBtn.Disable()

	ui.progress = widget.NewProgressBar()
	ui.progress.Hide()
	ui.statusLabel = wrappedLabel("")

	runSection := section("3. Run",
		ui.dryRunCheck,
		container.NewHBox(ui.runBtn, ui.exportBtn, ui.openOutBtn),
		ui.progress,
		ui.statusLabel,
	)

	// ── Console ──
	ui.console = newConsoleView()
	consoleTitle := widget.NewLabelWithStyle("Progress", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	top := container.NewVBox(header, templateSection, devicesSection, runSection, consoleTitle)
	ui.root = container.NewPadded(container.NewBorder(top, nil, nil, nil, ui.console.scroll))
	return ui
}

// section is a framed group with a normal-size bold title. widget.Card's
// own title renders at heading size, which outweighs the app name; an
// untitled Card gives the frame and the title is just a bold label inside.
func section(title string, content ...fyne.CanvasObject) fyne.CanvasObject {
	items := append([]fyne.CanvasObject{widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})}, content...)
	return widget.NewCard("", "", container.NewVBox(items...))
}

// displayPath puts the file name first so that, when the label truncates,
// what disappears is the tail of the folder, not the name.
func displayPath(p string) string {
	return filepath.Base(p) + "  —  " + filepath.Dir(p)
}

// pathLabel is a single-line label that truncates long paths with an
// ellipsis instead of growing the window.
func pathLabel(placeholder string) *widget.Label {
	l := widget.NewLabel(placeholder)
	l.Truncation = fyne.TextTruncateEllipsis
	return l
}

// wrappedLabel is the regular face on purpose: Fyne's bundled monospace
// clips underscores in labels, and variable names are full of them. It
// starts hidden: an empty label still takes a row, and every section
// would open with a blank gap. setNote shows it with its first text.
func wrappedLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	if text == "" {
		l.Hide()
	}
	return l
}

// setNote sets a wrappedLabel's text and hides it again when cleared.
// Showing or hiding a label changes its section's height, and Show() alone
// does not re-run the enclosing layouts, so the whole tree is refreshed
// when visibility flips (rare: first message, or clearing one).
func (ui *windowUI) setNote(l *widget.Label, text string) {
	l.SetText(text)
	wasVisible := l.Visible()
	if text == "" {
		l.Hide()
	} else {
		l.Show()
	}
	if l.Visible() != wasVisible {
		ui.root.Refresh()
	}
}

// pickFile opens the native file dialog filtered to exts, starting in the
// folder of the previously chosen file so the second pick is one click.
func (ui *windowUI) pickFile(exts []string, onPick func(path string)) {
	fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil || reader == nil {
			return
		}
		defer reader.Close()
		onPick(reader.URI().Path())
	}, ui.w)
	fd.SetFilter(storage.NewExtensionFileFilter(exts))
	if start := firstNonEmpty(ui.excelPath, ui.templatePath); start != "" {
		if lister, err := storage.ListerForURI(storage.NewFileURI(filepath.Dir(start))); err == nil {
			fd.SetLocation(lister)
		}
	}
	fd.Resize(fyne.NewSize(760, 520))
	fd.Show()
}

// ─── 1. Template ───────────────────────────────────────────────────────────

// loadTemplate parses the chosen template and reports its shape. It is the
// gate for everything else: without a parsed template there is nothing to
// generate an Excel from and nothing to run.
func (ui *windowUI) loadTemplate() {
	ui.tmpl = nil
	ui.generateBtn.Disable()
	tmpl, err := template.Load(ui.templatePath)
	if err != nil {
		ui.setNote(ui.templateSummary, "✗ Template: "+err.Error())
		ui.refreshActions()
		return
	}
	ui.tmpl = tmpl
	ui.setNote(ui.templateSummary, fmt.Sprintf("✓ %d lines · %d variables: %s",
		len(tmpl.Lines), len(tmpl.Variables), strings.Join(tmpl.Variables, ", ")))
	ui.generateBtn.Enable()
	// An Excel chosen for a previous template may not match this one.
	if ui.excelPath != "" {
		ui.checkDevices()
	}
	ui.refreshActions()
}

// generateExcel writes the devices workbook for the template next to it
// (never overwriting an existing one) and makes it the chosen Excel.
func (ui *windowUI) generateExcel() {
	if ui.tmpl == nil {
		return
	}
	path := excelsheet.DefaultExcelPath(ui.templatePath)
	if err := excelsheet.Generate(path, ui.tmpl.Variables); err != nil {
		dialog.ShowError(fmt.Errorf("could not create the Excel file: %w", err), ui.w)
		return
	}
	ui.setExcel(path)
	ui.setNote(ui.devicesSummary, fmt.Sprintf(
		"✓ %s created next to the template with the header row (%d variable columns). Fill one row per device, save, then Run — the file is re-read every time.",
		filepath.Base(path), len(ui.tmpl.Variables)))
	ui.refreshActions()
}

// ─── 2. Devices ────────────────────────────────────────────────────────────

func (ui *windowUI) setExcel(path string) {
	ui.excelPath = path
	ui.excelLabel.SetText(displayPath(path))
	ui.openExcelBtn.Enable()
}

// checkDevices re-reads the Excel and reports how many devices it holds,
// or the loader's error. Informational: Run/Export read the file again.
func (ui *windowUI) checkDevices() {
	devices, err := ui.loadDevices()
	if err != nil {
		ui.setNote(ui.devicesSummary, "✗ Excel: "+err.Error())
	} else {
		ui.setNote(ui.devicesSummary, fmt.Sprintf("✓ %d devices", len(devices)))
	}
	ui.refreshActions()
}

// loadDevices is the single place both actions get the batch from, so a
// sheet edited after being chosen is always read fresh.
func (ui *windowUI) loadDevices() ([]excelsheet.Device, error) {
	if ui.tmpl == nil {
		return nil, fmt.Errorf("choose a valid template first")
	}
	devices, err := excelsheet.Load(ui.excelPath, ui.tmpl.Variables)
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("the Excel file has no device rows")
	}
	return devices, nil
}

// refreshActions enables Run/Export once a template is parsed and an Excel
// is chosen. Whether the Excel is *filled* is checked when the action
// runs, because the user fills it after choosing it.
func (ui *windowUI) refreshActions() {
	if ui.tmpl != nil && ui.excelPath != "" {
		ui.runBtn.Enable()
		ui.exportBtn.Enable()
		return
	}
	ui.runBtn.Disable()
	ui.exportBtn.Disable()
}

// outDir is always next to the Excel, for runs and exports alike.
func (ui *windowUI) outDir() (string, error) {
	dir := filepath.Join(filepath.Dir(ui.excelPath), "smartconfigure-output")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("could not create the output folder: %w", err)
	}
	ui.lastOutDir = dir
	return dir, nil
}

// ─── 3. Run ────────────────────────────────────────────────────────────────

// prepareBatch re-reads the Excel for an action and reports a loader error
// in the Devices section, where the user can act on it.
func (ui *windowUI) prepareBatch(action string) ([]excelsheet.Device, bool) {
	devices, err := ui.loadDevices()
	if err != nil {
		ui.setNote(ui.devicesSummary, "✗ Excel: "+err.Error())
		ui.setNote(ui.statusLabel, "Nothing was "+action+": fix the Excel first.")
		return nil, false
	}
	ui.setNote(ui.devicesSummary, fmt.Sprintf("✓ %d devices", len(devices)))
	return devices, true
}

func (ui *windowUI) run() {
	devices, ok := ui.prepareBatch("run")
	if !ok {
		return
	}
	lines := ui.tmpl.Lines
	dryRun := ui.dryRunCheck.Checked

	ui.setBusy(true)
	ui.console.Clear()
	ui.progress.SetValue(0)
	ui.showProgress(true)

	go func() {
		defer fyne.Do(func() { ui.setBusy(false) })

		outDir, err := ui.outDir()
		if err != nil {
			ui.console.Append("ERROR: " + err.Error())
			return
		}

		mode := "LIVE (connecting to the devices)"
		if dryRun {
			mode = "DRY-RUN (no connection)"
		}
		ui.console.Append("Mode: " + mode)
		ui.console.Append(fmt.Sprintf("%d device(s), %d template line(s)", len(devices), len(lines)))
		ui.console.Append("")
		ui.setStatus(fmt.Sprintf("Running 0/%d…", len(devices)))

		total := len(devices)
		results, err := batch.Run(batch.Options{
			Devices: devices,
			Lines:   lines,
			OutDir:  outDir,
			DryRun:  dryRun,
			SSH:     defaultSSH,
			OnProgress: func(p batch.Progress) {
				dur := p.Result.Duration.Round(time.Millisecond)
				if p.Result.Success {
					ui.console.Append(fmt.Sprintf("[%d/%d] %s ... OK (%s)", p.Index, p.Total, p.Device, dur))
				} else {
					ui.console.Append(fmt.Sprintf("[%d/%d] %s ... FAILED (%s): %s", p.Index, p.Total, p.Device, dur, p.Result.Error))
				}
				fyne.Do(func() { ui.progress.SetValue(float64(p.Index) / float64(total)) })
				ui.setStatus(fmt.Sprintf("Running %d/%d…", p.Index, total))
			},
		})
		if err != nil {
			ui.console.Append("ERROR: " + err.Error())
			return
		}

		ok := 0
		for _, r := range results {
			if r.Success {
				ok++
			}
		}
		ui.console.Append("")
		ui.console.Append(fmt.Sprintf("Done: %d/%d succeeded.", ok, len(results)))
		ui.console.Append("Logs and report.csv saved in:")
		ui.console.Append(outDir)

		summary := fmt.Sprintf("Done: %d OK, %d failed.", ok, len(results)-ok)
		if dryRun {
			summary = fmt.Sprintf("Dry-run done: %d OK, %d with missing values.", ok, len(results)-ok)
		}
		ui.setStatus(summary)
		fyne.Do(func() { ui.openOutBtn.Enable() })
	}()
}

// exportSecureCRT writes the SecureCRT scripts for the batch. Pure file
// output: it never connects, so it is allowed with Dry-run ticked.
func (ui *windowUI) exportSecureCRT() {
	devices, ok := ui.prepareBatch("exported")
	if !ok {
		return
	}
	lines := ui.tmpl.Lines

	ui.setBusy(true)
	ui.console.Clear()
	ui.showProgress(false)

	go func() {
		defer fyne.Do(func() { ui.setBusy(false) })

		rendered, err := securecrt.Build(devices, lines)
		if err != nil {
			ui.console.Append("ERROR: " + err.Error())
			ui.setStatus("Nothing was exported: fix the Excel first.")
			return
		}
		outDir, err := ui.outDir()
		if err != nil {
			ui.console.Append("ERROR: " + err.Error())
			return
		}
		paths, err := securecrt.Write(securecrt.Batch{
			Version:      ui.version,
			GeneratedAt:  time.Now(),
			TemplatePath: ui.templatePath,
			ExcelPath:    ui.excelPath,
			Port:         defaultSSH.Port,
			IdleTimeout:  defaultSSH.IdleTimeout,
			LineTimeout:  defaultSSH.LineTimeout,
			Devices:      rendered,
		}, outDir)
		if err != nil {
			ui.console.Append("ERROR: could not write the script: " + err.Error())
			return
		}

		ui.console.Append(fmt.Sprintf("SecureCRT script written for %d device(s):", len(rendered)))
		for _, p := range paths {
			ui.console.Append("  " + p)
		}
		ui.console.Append("")
		ui.console.Append("How to use: open SecureCRT → Script → Run… → pick the .vbs (Windows) or the .py.")
		ui.console.Append("The script connects to each device, sends the template and leaves <IP>.securecrt.log next to itself.")
		ui.console.Append("")
		ui.console.Append("WARNING: the script contains the usernames and passwords from the Excel. Do not share or upload it.")
		ui.setStatus(fmt.Sprintf("Script exported (%d devices). Nothing was sent to any device.", len(rendered)))
		fyne.Do(func() { ui.openOutBtn.Enable() })
	}()
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func (ui *windowUI) setBusy(busy bool) {
	if busy {
		ui.runBtn.Disable()
		ui.exportBtn.Disable()
		ui.openOutBtn.Disable()
		return
	}
	ui.refreshActions()
}

// showProgress toggles the bar and re-lays out the Run section, for the
// same reason as setNote.
func (ui *windowUI) showProgress(show bool) {
	if show == ui.progress.Visible() {
		return
	}
	if show {
		ui.progress.Show()
	} else {
		ui.progress.Hide()
	}
	ui.root.Refresh()
}

// setStatus is safe to call from any goroutine.
func (ui *windowUI) setStatus(text string) {
	fyne.Do(func() { ui.setNote(ui.statusLabel, text) })
}

func absDiff(a, b float32) float32 {
	if a > b {
		return a - b
	}
	return b - a
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// openPath opens a folder in the file manager or a file in its default
// application (the generated .xlsx in Excel). Best-effort: errors are
// ignored since this is a convenience shortcut — every path is also
// printed in the window.
func openPath(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	_ = cmd.Start()
}

// ─── Console ───────────────────────────────────────────────────────────────

// consoleView is the read-only, monospace progress log. A TextGrid rather
// than a disabled Entry: it is not greyed out, it is genuinely
// monospaced, and single rows can be coloured (OK green, FAILED/ERROR
// red) without touching the rest of the text.
type consoleView struct {
	grid   *widget.TextGrid
	scroll *container.Scroll
	lines  []string
}

func newConsoleView() *consoleView {
	c := &consoleView{grid: widget.NewTextGrid()}
	c.scroll = container.NewScroll(c.grid)
	c.scroll.SetMinSize(fyne.NewSize(0, 200))
	return c
}

// Append adds one line and repaints; safe to call from any goroutine.
func (c *consoleView) Append(line string) {
	fyne.Do(func() {
		c.lines = append(c.lines, line)
		c.grid.SetText(strings.Join(c.lines, "\n"))
		c.styleRows()
		c.grid.Refresh()
		c.scroll.ScrollToBottom()
	})
}

func (c *consoleView) Clear() {
	c.lines = nil
	c.grid.SetText("")
	c.grid.Refresh()
}

// styleRows colours outcome lines. Colour is reinforcement only: the
// words OK / FAILED / ERROR are always there (MASTER.md §2, "status is words").
func (c *consoleView) styleRows() {
	okStyle := &widget.CustomTextGridStyle{FGColor: theme.Color(theme.ColorNameSuccess)}
	failStyle := &widget.CustomTextGridStyle{FGColor: theme.Color(theme.ColorNameError)}
	for i, l := range c.lines {
		switch {
		case strings.Contains(l, "... OK ("):
			c.grid.SetRowStyle(i, okStyle)
		case strings.Contains(l, "... FAILED (") || strings.HasPrefix(l, "ERROR:") || strings.HasPrefix(l, "WARNING:"):
			c.grid.SetRowStyle(i, failStyle)
		}
	}
}

// ─── Form layout ───────────────────────────────────────────────────────────

// formLayout lays out (button, label) pairs in two columns: the buttons
// take their natural width, the labels take the rest. Fyne's GridWithColumns
// would split 50/50 and waste half the row on the button.
type formLayout struct{}

func newFormLayout() fyne.Layout { return &formLayout{} }

func (f *formLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var left, height float32
	for i := 0; i+1 < len(objects); i += 2 {
		bs, ls := objects[i].MinSize(), objects[i+1].MinSize()
		if bs.Width > left {
			left = bs.Width
		}
		row := bs.Height
		if ls.Height > row {
			row = ls.Height
		}
		height += row + theme.Padding()
	}
	return fyne.NewSize(left+theme.Padding()+120, height)
}

func (f *formLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	var left float32
	for i := 0; i < len(objects); i += 2 {
		if w := objects[i].MinSize().Width; w > left {
			left = w
		}
	}
	var y float32
	for i := 0; i+1 < len(objects); i += 2 {
		btn, lbl := objects[i], objects[i+1]
		row := btn.MinSize().Height
		if h := lbl.MinSize().Height; h > row {
			row = h
		}
		btn.Resize(fyne.NewSize(left, row))
		btn.Move(fyne.NewPos(0, y))
		lbl.Resize(fyne.NewSize(size.Width-left-theme.Padding(), row))
		lbl.Move(fyne.NewPos(left+theme.Padding(), y))
		y += row + theme.Padding()
	}
}
