// Package securecrt turns a loaded template + device list into a script
// that SecureCRT (VanDyke) runs on its own: it connects to every device
// over SSH2, sends the rendered template line by line, keeps a per-device
// log, and continues with the next device when one fails.
//
// SmartConfigure itself never connects here — this package only writes
// files. It exists for workstations where SecureCRT is the only client
// allowed to touch the devices. Two flavours are produced from the same
// Batch so they can never drift: VBScript (works on every Windows
// SecureCRT) and Python 3 (SecureCRT builds with bundled Python, macOS,
// Linux). Design contract: design-system/smartconfigure/pages/desktop.md.
package securecrt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/javimcasas/smartconfigure/internal/excelsheet"
	"github.com/javimcasas/smartconfigure/internal/template"
)

// File names written next to the logs in the output folder.
const (
	VBScriptFileName = "smartconfigure-securecrt.vbs"
	PythonFileName   = "smartconfigure-securecrt.py"
)

// Device is one row of the Excel with the template already rendered for it.
type Device struct {
	Host     string
	User     string
	Password string
	Lines    []string
}

// Batch is everything the generated scripts embed. Timings mirror the
// sshrunner.Config of a direct run so a SecureCRT run behaves the same.
type Batch struct {
	Version      string    // SmartConfigure version, for the header comment
	GeneratedAt  time.Time // for the header comment; tests pin it
	TemplatePath string    // informational only
	ExcelPath    string    // informational only
	Port         int
	IdleTimeout  time.Duration // pause after each line
	LineTimeout  time.Duration // max wait for a prompt after each line
	Devices      []Device
}

// Build renders lines for every device (same template.Render as a dry
// run) and refuses to continue on the first unresolved {VARIABLE}: a
// script that sends a literal placeholder to a switch is worse than no
// script, so the error text matches the dry-run one on purpose.
func Build(devices []excelsheet.Device, lines []string) ([]Device, error) {
	out := make([]Device, 0, len(devices))
	for _, dev := range devices {
		rendered := make([]string, 0, len(lines))
		for i, raw := range lines {
			line := template.Render(raw, dev.Vars)
			if template.HasUnresolvedPlaceholder(line) {
				return nil, fmt.Errorf("device %s: line %d has a variable with no value in the Excel row: %s", dev.IP, i+1, line)
			}
			rendered = append(rendered, line)
		}
		out = append(out, Device{Host: dev.IP, User: dev.Username, Password: dev.Password, Lines: rendered})
	}
	return out, nil
}

// Write renders both scripts into outDir and returns their paths.
func Write(b Batch, outDir string) ([]string, error) {
	if len(b.Devices) == 0 {
		return nil, fmt.Errorf("nothing to export: the Excel has no device rows")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("could not create output directory: %w", err)
	}
	paths := []string{
		filepath.Join(outDir, VBScriptFileName),
		filepath.Join(outDir, PythonFileName),
	}
	if err := os.WriteFile(paths[0], RenderVBScript(b), 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(paths[1], RenderPython(b), 0o600); err != nil {
		return nil, err
	}
	return paths, nil
}

// ─── Shared header ─────────────────────────────────────────────────────────

// headerLines is the comment block both flavours start with, minus the
// per-language comment marker.
func headerLines(b Batch) []string {
	return []string{
		"SmartConfigure - SecureCRT batch script",
		fmt.Sprintf("Generated %s by SmartConfigure %s", b.GeneratedAt.Format("2006-01-02 15:04:05"), b.Version),
		fmt.Sprintf("  template: %s", b.TemplatePath),
		fmt.Sprintf("  devices:  %s (%d row(s))", b.ExcelPath, len(b.Devices)),
		"",
		"Run it from SecureCRT: Script > Run... > pick this file. For each device it",
		"opens an SSH2 session, sends the commands below one line at a time (waiting",
		"for the device prompt after each one), logs the whole session to",
		"<host>.securecrt.log next to this script, disconnects and moves on. A device",
		"that fails to connect is skipped, the rest of the batch continues.",
		"",
		"WARNING: this file contains the usernames and passwords from the Excel.",
		"Treat it like the Excel itself - do not share, upload or commit it.",
	}
}

// ─── VBScript ──────────────────────────────────────────────────────────────

// RenderVBScript returns the batch as a SecureCRT VBScript (CRLF, ASCII —
// non-ASCII characters are emitted as ChrW() so the file never depends on
// the Windows code page).
func RenderVBScript(b Batch) []byte {
	var w strings.Builder
	line := func(format string, args ...interface{}) {
		fmt.Fprintf(&w, format, args...)
		w.WriteString("\r\n")
	}

	line(`#$language = "VBScript"`)
	line(`#$interface = "1.0"`)
	line("")
	for _, h := range headerLines(b) {
		if h == "" {
			line("'")
		} else {
			line("' %s", h)
		}
	}
	line("")
	line("Option Explicit")
	line("")
	line("Const PORT = %d", b.Port)
	line("Const IDLE_MS = %d", b.IdleTimeout.Milliseconds())
	line("Const LINE_TIMEOUT_S = %d", secondsCeil(b.LineTimeout))
	line("")
	line("Sub Main")
	line("  Dim logDir")
	line(`  logDir = Left(crt.ScriptFullName, InStrRev(crt.ScriptFullName, "\"))`)
	line("")
	line("  Dim devices(%d)", len(b.Devices)-1)
	for i, d := range b.Devices {
		line("  devices(%d) = Array(%s, %s, %s, Array( _", i, vbString(d.Host), vbString(d.User), vbString(d.Password))
		for j, l := range d.Lines {
			sep := ", _"
			if j == len(d.Lines)-1 {
				sep = " _"
			}
			line("    %s%s", vbString(l), sep)
		}
		line("  ))")
	}
	line("")
	line("  Dim i, ok, failed")
	line("  ok = 0 : failed = 0")
	line("  For i = 0 To UBound(devices)")
	line("    If RunDevice(devices(i)(0), devices(i)(1), devices(i)(2), devices(i)(3), logDir) Then")
	line("      ok = ok + 1")
	line("    Else")
	line("      failed = failed + 1")
	line("    End If")
	line("  Next")
	line("")
	line(`  crt.Dialog.MessageBox "SmartConfigure batch finished: " & ok & " OK, " & failed & " failed." & vbCrLf & "Logs: " & logDir`)
	line("End Sub")
	line("")
	line("' Connects, sends every line, logs, disconnects. Returns False when the")
	line("' connection could not be established; the caller moves on to the next device.")
	line("Function RunDevice(host, user, pass, lines, logDir)")
	line("  RunDevice = False")
	line("")
	line("  On Error Resume Next")
	line(`  crt.Session.Connect "/SSH2 /L " & Chr(34) & user & Chr(34) & " /PASSWORD " & Chr(34) & pass & Chr(34) & " /P " & PORT & " /ACCEPTHOSTKEYS " & host`)
	line("  If Err.Number <> 0 Or Not crt.Session.Connected Then")
	line("    Err.Clear")
	line("    On Error GoTo 0")
	line("    Exit Function")
	line("  End If")
	line("  On Error GoTo 0")
	line("")
	line(`  crt.Session.LogFileName = logDir & host & ".securecrt.log"`)
	line("  crt.Session.Log True")
	line("  crt.Screen.Synchronous = True")
	line("")
	line("  ' Login banner / first prompt. VRP prompts end in > (user view) or ] (system view).")
	line(`  crt.Screen.WaitForStrings ">", "]", LINE_TIMEOUT_S`)
	line("")
	line("  Dim i")
	line("  For i = 0 To UBound(lines)")
	line("    crt.Screen.Send lines(i) & vbCr")
	line(`    crt.Screen.WaitForStrings ">", "]", LINE_TIMEOUT_S`)
	line("    crt.Sleep IDLE_MS")
	line("  Next")
	line("")
	line("  crt.Session.Log False")
	line("  crt.Session.Disconnect")
	line("  Do While crt.Session.Connected")
	line("    crt.Sleep 100")
	line("  Loop")
	line("  crt.Sleep 500")
	line("  RunDevice = True")
	line("End Function")

	return []byte(w.String())
}

// vbString quotes s as a VBScript string literal: " becomes "", and any
// non-ASCII rune is spliced in with ChrW() so the file stays pure ASCII.
func vbString(s string) string {
	var w strings.Builder
	w.WriteByte('"')
	open := true // inside a "..." literal
	for _, r := range s {
		switch {
		case r == '"':
			if !open {
				w.WriteString(` & "`)
				open = true
			}
			w.WriteString(`""`)
		case r < 0x20 || r > 0x7e:
			if open {
				w.WriteString(`" & `)
				open = false
			} else {
				w.WriteString(" & ")
			}
			fmt.Fprintf(&w, "ChrW(%d)", r)
		default:
			if !open {
				w.WriteString(` & "`)
				open = true
			}
			w.WriteRune(r)
		}
	}
	if open {
		w.WriteByte('"')
	}
	return w.String()
}

// ─── Python 3 ──────────────────────────────────────────────────────────────

// RenderPython returns the batch as a SecureCRT Python 3 script (CRLF,
// ASCII — non-ASCII characters are \u-escaped).
func RenderPython(b Batch) []byte {
	var w strings.Builder
	line := func(format string, args ...interface{}) {
		fmt.Fprintf(&w, format, args...)
		w.WriteString("\r\n")
	}

	line(`#$language = "Python3"`)
	line(`#$interface = "1.0"`)
	line("")
	for _, h := range headerLines(b) {
		if h == "" {
			line("#")
		} else {
			line("# %s", h)
		}
	}
	line("")
	line("import os")
	line("")
	line("PORT = %d", b.Port)
	line("IDLE_MS = %d", b.IdleTimeout.Milliseconds())
	line("LINE_TIMEOUT_S = %d", secondsCeil(b.LineTimeout))
	line("")
	line("DEVICES = [")
	for _, d := range b.Devices {
		line("    {")
		line(`        "host": %s,`, pyString(d.Host))
		line(`        "user": %s,`, pyString(d.User))
		line(`        "password": %s,`, pyString(d.Password))
		line(`        "lines": [`)
		for _, l := range d.Lines {
			line("            %s,", pyString(l))
		}
		line("        ],")
		line("    },")
	}
	line("]")
	line("")
	line("")
	line("def run_device(dev, log_dir):")
	line(`    """Connects, sends every line, logs, disconnects. False when the`)
	line(`    connection could not be established; the caller moves on."""`)
	line("    try:")
	line(`        crt.Session.Connect('/SSH2 /L "%%s" /PASSWORD "%%s" /P %%d /ACCEPTHOSTKEYS %%s'`)
	line(`                            %% (dev["user"], dev["password"], PORT, dev["host"]))`)
	line("    except Exception:")
	line("        return False")
	line("    if not crt.Session.Connected:")
	line("        return False")
	line("")
	line(`    crt.Session.LogFileName = os.path.join(log_dir, dev["host"] + ".securecrt.log")`)
	line("    crt.Session.Log(True)")
	line("    crt.Screen.Synchronous = True")
	line("")
	line("    # Login banner / first prompt. VRP prompts end in > (user view) or ] (system view).")
	line(`    crt.Screen.WaitForStrings([">", "]"], LINE_TIMEOUT_S)`)
	line("")
	line(`    for cmd in dev["lines"]:`)
	line(`        crt.Screen.Send(cmd + "\r")`)
	line(`        crt.Screen.WaitForStrings([">", "]"], LINE_TIMEOUT_S)`)
	line("        crt.Sleep(IDLE_MS)")
	line("")
	line("    crt.Session.Log(False)")
	line("    crt.Session.Disconnect()")
	line("    while crt.Session.Connected:")
	line("        crt.Sleep(100)")
	line("    crt.Sleep(500)")
	line("    return True")
	line("")
	line("")
	line("def main():")
	line("    log_dir = os.path.dirname(crt.ScriptFullName)")
	line("    ok = failed = 0")
	line("    for dev in DEVICES:")
	line("        if run_device(dev, log_dir):")
	line("            ok += 1")
	line("        else:")
	line("            failed += 1")
	line(`    crt.Dialog.MessageBox("SmartConfigure batch finished: %%d OK, %%d failed.\nLogs: %%s" %% (ok, failed, log_dir))`)
	line("")
	line("")
	line("main()")

	return []byte(w.String())
}

// pyString quotes s as a Python string literal with only ASCII bytes.
func pyString(s string) string {
	var w strings.Builder
	w.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			w.WriteByte('\\')
			w.WriteRune(r)
		case r == '\t':
			w.WriteString(`\t`)
		case r < 0x20 || r > 0x7e:
			if r > 0xffff {
				fmt.Fprintf(&w, `\U%08x`, r)
			} else {
				fmt.Fprintf(&w, `\u%04x`, r)
			}
		default:
			w.WriteRune(r)
		}
	}
	w.WriteByte('"')
	return w.String()
}

// secondsCeil converts a duration to whole seconds, never below 1 —
// SecureCRT's WaitForStrings timeout is an integer number of seconds.
func secondsCeil(d time.Duration) int {
	s := int((d + time.Second - 1) / time.Second)
	if s < 1 {
		return 1
	}
	return s
}
