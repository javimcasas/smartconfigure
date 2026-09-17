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

//go:embed icon.svg
var iconSVG []byte

// defaultSSH is what the window uses for a real run and for the SecureCRT
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
// entry point when the binary is run with no --template/--excel flags
// (i.e. double-clicked from Explorer/Finder), so most users never need to
// touch a terminal at all. It shares the exact same batch.Run() engine and
// securecrt generator as the CLI path in main.go, so behaviour never
// drifts between the two.
func Launch(version string) {
	a := app.New()
	a.Settings().SetTheme(scTheme{theme.DefaultTheme()})
	icon := fyne.NewStaticResource("smartconfigure.svg", iconSVG)
	a.SetIcon(icon)

	w := a.NewWindow("SmartConfigure " + version)
	w.SetIcon(icon)
	w.Resize(fyne.NewSize(860, 680))
	w.CenterOnScreen()

	ui := newWindowUI(w, version)
	w.SetContent(ui.root)
	w.ShowAndRun()
}

// windowUI holds every widget the handlers need. State lives here, not in
// closures, so the three actions (validate, run, export) share it.
type windowUI struct {
	w       fyne.Window
	version string

	templatePath, excelPath, lastOutDir string
	loaded                              *loadedInputs // set by validate(), nil until both files parse

	templateLabel, excelLabel, summaryLabel *widget.Label
	dryRunCheck                            *widget.Check
	runBtn, exportBtn, openOutputBtn       *widget.Button
	progress                               *widget.ProgressBar
	statusLabel                            *widget.Label
	console                                *consoleView

	root fyne.CanvasObject
}

// loadedInputs is the parsed template + Excel, kept so Run/Export don't
// re-read the files after validate() already reported on them.
type loadedInputs struct {
	tmpl    *template.Template
	devices []excelsheet.Device
}

func newWindowUI(w fyne.Window, version string) *windowUI {
	ui := &windowUI{w: w, version: version}

	// ── Header ──
	title := widget.NewLabelWithStyle("SmartConfigure", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabel("Ejecuta un template de comandos SSH sobre varios equipos definidos en un Excel.")
	subtitle.Wrapping = fyne.TextWrapWord
	header := container.NewVBox(title, subtitle)

	// ── Files card ──
	ui.templateLabel = pathLabel("Ningún template seleccionado")
	ui.excelLabel = pathLabel("Ningún Excel seleccionado")
	ui.summaryLabel = widget.NewLabel("")
	ui.summaryLabel.Wrapping = fyne.TextWrapWord
	ui.summaryLabel.TextStyle = fyne.TextStyle{Monospace: true}

	pickTemplateBtn := widget.NewButtonWithIcon("Elegir template (.txt)…", theme.DocumentIcon(), func() {
		ui.pickFile([]string{".txt"}, func(p string) {
			ui.templatePath = p
			ui.templateLabel.SetText(p)
			ui.validate()
		})
	})
	pickExcelBtn := widget.NewButtonWithIcon("Elegir Excel (.xlsx)…", theme.ListIcon(), func() {
		ui.pickFile([]string{".xlsx"}, func(p string) {
			ui.excelPath = p
			ui.excelLabel.SetText(p)
			ui.validate()
		})
	})
	filesGrid := container.New(newFormLayout(),
		pickTemplateBtn, ui.templateLabel,
		pickExcelBtn, ui.excelLabel,
	)
	filesCard := widget.NewCard("Archivos", "", container.NewVBox(filesGrid, ui.summaryLabel))

	// ── Run card ──
	ui.dryRunCheck = widget.NewCheck("Dry-run (no conectar a los equipos, solo validar)", nil)
	ui.dryRunCheck.SetChecked(true)

	ui.runBtn = widget.NewButtonWithIcon("Ejecutar", theme.MediaPlayIcon(), ui.run)
	ui.runBtn.Importance = widget.HighImportance
	ui.runBtn.Disable()

	ui.exportBtn = widget.NewButtonWithIcon("Exportar script SecureCRT", theme.DownloadIcon(), ui.exportSecureCRT)
	ui.exportBtn.Importance = widget.MediumImportance
	ui.exportBtn.Disable()

	ui.openOutputBtn = widget.NewButtonWithIcon("Abrir carpeta de resultados", theme.FolderOpenIcon(), func() {
		if ui.lastOutDir != "" {
			openFolder(ui.lastOutDir)
		}
	})
	ui.openOutputBtn.Importance = widget.LowImportance
	ui.openOutputBtn.Disable()

	actions := container.NewHBox(ui.runBtn, ui.exportBtn, ui.openOutputBtn)

	ui.progress = widget.NewProgressBar()
	ui.progress.Hide()
	ui.statusLabel = widget.NewLabel("")
	ui.statusLabel.Wrapping = fyne.TextWrapWord

	runCard := widget.NewCard("Ejecución", "", container.NewVBox(ui.dryRunCheck, actions, ui.progress, ui.statusLabel))

	// ── Console ──
	ui.console = newConsoleView()
	consoleTitle := widget.NewLabelWithStyle("Progreso", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})

	top := container.NewVBox(header, filesCard, runCard, consoleTitle)
	ui.root = container.NewPadded(container.NewBorder(top, nil, nil, nil, ui.console.scroll))
	return ui
}

// pathLabel is a single-line label that truncates long paths with an
// ellipsis instead of growing the window.
func pathLabel(placeholder string) *widget.Label {
	l := widget.NewLabel(placeholder)
	l.Truncation = fyne.TextTruncateEllipsis
	return l
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

// ─── Validate ──────────────────────────────────────────────────────────────

// validate parses both files as soon as they are chosen and reports the
// shape of the batch (or the first problem) before the user presses
// anything. Run and Export are only enabled once both parse.
func (ui *windowUI) validate() {
	ui.loaded = nil
	ui.runBtn.Disable()
	ui.exportBtn.Disable()
	if ui.templatePath == "" || ui.excelPath == "" {
		ui.summaryLabel.SetText("")
		return
	}

	tmpl, err := template.Load(ui.templatePath)
	if err != nil {
		ui.setSummary("✗ Template: " + err.Error())
		return
	}
	devices, err := excelsheet.Load(ui.excelPath, tmpl.Variables)
	if err != nil {
		ui.setSummary("✗ Excel: " + err.Error())
		return
	}
	if len(devices) == 0 {
		ui.setSummary("✗ El Excel no tiene ninguna fila de equipos.")
		return
	}

	ui.loaded = &loadedInputs{tmpl: tmpl, devices: devices}
	ui.setSummary(fmt.Sprintf("✓ %d equipo(s) · %d línea(s) · %d variable(s): %s",
		len(devices), len(tmpl.Lines), len(tmpl.Variables), strings.Join(tmpl.Variables, ", ")))
	ui.runBtn.Enable()
	ui.exportBtn.Enable()
}

func (ui *windowUI) setSummary(text string) {
	ui.summaryLabel.SetText(text)
}

// outDir is always next to the Excel, for runs and exports alike.
func (ui *windowUI) outDir() (string, error) {
	dir := filepath.Join(filepath.Dir(ui.excelPath), "smartconfigure-output")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("no se pudo crear la carpeta de salida: %w", err)
	}
	ui.lastOutDir = dir
	return dir, nil
}

// ─── Run ───────────────────────────────────────────────────────────────────

func (ui *windowUI) run() {
	if ui.loaded == nil {
		dialog.ShowInformation("Faltan archivos", "Elige primero un template y un Excel válidos.", ui.w)
		return
	}
	in := ui.loaded
	dryRun := ui.dryRunCheck.Checked

	ui.setBusy(true)
	ui.console.Clear()
	ui.progress.SetValue(0)
	ui.progress.Show()

	go func() {
		defer fyne.Do(func() { ui.setBusy(false) })

		outDir, err := ui.outDir()
		if err != nil {
			ui.console.Append("ERROR: " + err.Error())
			return
		}

		mode := "REAL (conectará a los equipos)"
		if dryRun {
			mode = "DRY-RUN (no conecta a nada)"
		}
		ui.console.Append(fmt.Sprintf("Modo: %s", mode))
		ui.console.Append(fmt.Sprintf("%d equipo(s), %d línea(s) de template", len(in.devices), len(in.tmpl.Lines)))
		ui.console.Append("")
		ui.setStatus(fmt.Sprintf("Ejecutando 0/%d…", len(in.devices)))

		total := len(in.devices)
		results, err := batch.Run(batch.Options{
			Devices: in.devices,
			Lines:   in.tmpl.Lines,
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
				ui.setStatus(fmt.Sprintf("Ejecutando %d/%d…", p.Index, total))
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
		ui.console.Append(fmt.Sprintf("Hecho: %d/%d correctos.", ok, len(results)))
		ui.console.Append("Logs y report.csv guardados en:")
		ui.console.Append(outDir)

		summary := fmt.Sprintf("Hecho: %d OK, %d fallidos.", ok, len(results)-ok)
		if dryRun {
			summary = fmt.Sprintf("Dry-run terminado: %d OK, %d con variables sin valor.", ok, len(results)-ok)
		}
		ui.setStatus(summary)
		fyne.Do(func() { ui.openOutputBtn.Enable() })
	}()
}

// ─── Export ────────────────────────────────────────────────────────────────

// exportSecureCRT writes the SecureCRT scripts for the loaded batch. Pure
// file output: it never connects, so it is allowed with Dry-run ticked.
func (ui *windowUI) exportSecureCRT() {
	if ui.loaded == nil {
		dialog.ShowInformation("Faltan archivos", "Elige primero un template y un Excel válidos.", ui.w)
		return
	}
	in := ui.loaded

	ui.setBusy(true)
	ui.console.Clear()
	ui.progress.Hide()

	go func() {
		defer fyne.Do(func() { ui.setBusy(false) })

		rendered, err := securecrt.Build(in.devices, in.tmpl.Lines)
		if err != nil {
			ui.console.Append("ERROR: " + err.Error())
			ui.setStatus("No se ha exportado nada: revisa el Excel.")
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
			ui.console.Append("ERROR: no se pudo escribir el script: " + err.Error())
			return
		}

		ui.console.Append(fmt.Sprintf("Script SecureCRT generado para %d equipo(s):", len(rendered)))
		for _, p := range paths {
			ui.console.Append("  " + p)
		}
		ui.console.Append("")
		ui.console.Append("Cómo usarlo: abre SecureCRT → Script → Run… → elige el .vbs (Windows) o el .py.")
		ui.console.Append("El script conecta a cada equipo, envía el template y deja un <IP>.securecrt.log junto al script.")
		ui.console.Append("")
		ui.console.Append("AVISO: el script contiene los usuarios y contraseñas del Excel. No lo compartas ni lo subas a ningún sitio.")
		ui.setStatus(fmt.Sprintf("Script exportado (%d equipos). SmartConfigure no ha conectado a nada.", len(rendered)))
		fyne.Do(func() { ui.openOutputBtn.Enable() })
	}()
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func (ui *windowUI) setBusy(busy bool) {
	if busy {
		ui.runBtn.Disable()
		ui.exportBtn.Disable()
		ui.openOutputBtn.Disable()
		return
	}
	if ui.loaded != nil {
		ui.runBtn.Enable()
		ui.exportBtn.Enable()
	}
}

// setStatus is safe to call from any goroutine.
func (ui *windowUI) setStatus(text string) {
	fyne.Do(func() { ui.statusLabel.SetText(text) })
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// openFolder opens path in the OS's default file manager. Best-effort:
// errors are ignored since this is just a convenience shortcut, not a
// core feature — the output folder path is always shown in the console too.
func openFolder(path string) {
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
		case strings.Contains(l, "... FAILED (") || strings.HasPrefix(l, "ERROR:") || strings.HasPrefix(l, "AVISO:"):
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
