package gui

import (
	"fmt"
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
	"fyne.io/fyne/v2/widget"

	"github.com/javimcasas/smartconfigure/internal/batch"
	"github.com/javimcasas/smartconfigure/internal/excelsheet"
	"github.com/javimcasas/smartconfigure/internal/sshrunner"
	"github.com/javimcasas/smartconfigure/internal/template"
)

// Launch opens the SmartConfigure graphical interface. It's the default
// entry point when the binary is run with no --template/--excel flags
// (i.e. double-clicked from Explorer/Finder), so most users never need to
// touch a terminal at all. It shares the exact same batch.Run() engine as
// the CLI path in main.go, so behaviour never drifts between the two.
func Launch(version string) {
	a := app.New()
	w := a.NewWindow("SmartConfigure " + version)
	w.Resize(fyne.NewSize(720, 600))

	var templatePath, excelPath, lastOutDir string

	templateLabel := widget.NewLabel("Ningún template seleccionado")
	templateLabel.Wrapping = fyne.TextWrapWord
	excelLabel := widget.NewLabel("Ningún Excel seleccionado")
	excelLabel.Wrapping = fyne.TextWrapWord

	pickTemplateBtn := widget.NewButton("Elegir template (.txt)...", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			templatePath = reader.URI().Path()
			templateLabel.SetText(templatePath)
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".txt"}))
		fd.Show()
	})

	pickExcelBtn := widget.NewButton("Elegir Excel (.xlsx)...", func() {
		fd := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil || reader == nil {
				return
			}
			defer reader.Close()
			excelPath = reader.URI().Path()
			excelLabel.SetText(excelPath)
		}, w)
		fd.SetFilter(storage.NewExtensionFileFilter([]string{".xlsx"}))
		fd.Show()
	})

	dryRunCheck := widget.NewCheck("Dry-run (no conectar a los equipos, solo validar)", nil)
	dryRunCheck.SetChecked(true)

	logEntry := widget.NewMultiLineEntry()
	logEntry.Wrapping = fyne.TextWrapWord
	logEntry.Disable() // read-only console-style output
	logScroll := container.NewScroll(logEntry)
	logScroll.SetMinSize(fyne.NewSize(680, 320))

	appendLog := func(line string) {
		fyne.Do(func() {
			if logEntry.Text == "" {
				logEntry.SetText(line)
			} else {
				logEntry.SetText(logEntry.Text + "\n" + line)
			}
			logEntry.CursorRow = strings.Count(logEntry.Text, "\n")
			logEntry.Refresh()
		})
	}

	openOutputBtn := widget.NewButton("📂 Abrir carpeta de resultados", func() {
		if lastOutDir != "" {
			openFolder(lastOutDir)
		}
	})
	openOutputBtn.Disable()

	var runBtn *widget.Button
	runBtn = widget.NewButton("▶  Ejecutar", func() {
		if templatePath == "" || excelPath == "" {
			dialog.ShowInformation("Faltan archivos", "Elige primero un template y un Excel.", w)
			return
		}
		runBtn.Disable()
		openOutputBtn.Disable()
		logEntry.SetText("")

		go func() {
			defer fyne.Do(func() { runBtn.Enable() })

			tmpl, err := template.Load(templatePath)
			if err != nil {
				appendLog("ERROR: no se pudo leer el template: " + err.Error())
				return
			}
			devices, err := excelsheet.Load(excelPath, tmpl.Variables)
			if err != nil {
				appendLog("ERROR: no se pudo leer el Excel: " + err.Error())
				return
			}
			if len(devices) == 0 {
				appendLog("ERROR: el Excel no tiene ninguna fila de equipos.")
				return
			}

			outDir := filepath.Join(filepath.Dir(excelPath), "smartconfigure-output")
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				appendLog("ERROR: no se pudo crear la carpeta de salida: " + err.Error())
				return
			}
			lastOutDir = outDir

			mode := "REAL (conectará a los equipos)"
			if dryRunCheck.Checked {
				mode = "DRY-RUN (no conecta a nada)"
			}
			appendLog(fmt.Sprintf("Modo: %s\n%d equipo(s), %d línea(s) de template\n", mode, len(devices), len(tmpl.Lines)))

			results, err := batch.Run(batch.Options{
				Devices: devices,
				Lines:   tmpl.Lines,
				OutDir:  outDir,
				DryRun:  dryRunCheck.Checked,
				SSH: sshrunner.Config{
					Port:           22,
					ConnectTimeout: 10 * time.Second,
					IdleTimeout:    800 * time.Millisecond,
					LineTimeout:    15 * time.Second,
				},
				OnProgress: func(p batch.Progress) {
					if p.Result.Success {
						appendLog(fmt.Sprintf("[%d/%d] %s ... OK (%s)", p.Index, p.Total, p.Device, p.Result.Duration.Round(time.Millisecond)))
					} else {
						appendLog(fmt.Sprintf("[%d/%d] %s ... FAILED (%s): %s", p.Index, p.Total, p.Device, p.Result.Duration.Round(time.Millisecond), p.Result.Error))
					}
				},
			})
			if err != nil {
				appendLog("ERROR: " + err.Error())
				return
			}

			ok := 0
			for _, r := range results {
				if r.Success {
					ok++
				}
			}
			appendLog(fmt.Sprintf("\nHecho: %d/%d correctos.\nLogs y report.csv guardados en:\n%s", ok, len(results), outDir))
			fyne.Do(func() { openOutputBtn.Enable() })
		}()
	})

	content := container.NewVBox(
		widget.NewLabelWithStyle("SmartConfigure", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		widget.NewLabel("Ejecuta un template de comandos SSH sobre varios equipos definidos en un Excel."),
		widget.NewSeparator(),
		pickTemplateBtn, templateLabel,
		pickExcelBtn, excelLabel,
		dryRunCheck,
		container.NewGridWithColumns(2, runBtn, openOutputBtn),
		widget.NewSeparator(),
		widget.NewLabel("Progreso:"),
		logScroll,
	)

	w.SetContent(container.NewPadded(content))
	w.ShowAndRun()
}

// openFolder opens path in the OS's default file manager. Best-effort:
// errors are ignored since this is just a convenience shortcut, not a
// core feature — the output folder path is always shown in the log too.
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
