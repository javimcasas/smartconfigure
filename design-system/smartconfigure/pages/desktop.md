# Page: Desktop window (Fyne) — overrides MASTER

`internal/gui/gui.go` (`Launch`). This is a native Fyne window, so the web
tokens in MASTER §2–§4 **do not apply literally** — Fyne draws with its own
theme. What carries over is the *structure* and the *semantics*: one
primary action, dry-run first, status as words, literal text in mono.

## Purpose

The 90% path: pick two files, keep Dry-run ticked, press Run, read the
progress, open the folder. And the planned 10% path: export a SecureCRT
script that runs the same batch from a workstation where only SecureCRT
may touch the devices.

## Layout (v0.2.0)

```
┌ SmartConfigure v0.2.0 ────────────────────────────────────────────── ✕ ┐
│ SmartConfigure (bold)                                                   │
│ Ejecuta un template de comandos SSH sobre varios equipos …              │
│ ┌ Archivos ───────────────────────────────────────────────────────────┐ │
│ │ [📄 Elegir template (.txt)…]  template.txt — C:\…\lote-sept          │ │
│ │ [☰ Elegir Excel (.xlsx)…]     equipos.xlsx — C:\…\lote-sept          │ │
│ │ ✓ 12 equipo(s) · 44 línea(s) · 5 variable(s): SWITCH_NAME, …         │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│ ┌ Ejecución ──────────────────────────────────────────────────────────┐ │
│ │ ☑ Dry-run (no conectar a los equipos, solo validar)                 │ │
│ │ [▶ Ejecutar]  [⇩ Exportar script SecureCRT]  [📂 Abrir carpeta…]    │ │
│ │ ▓▓▓▓▓▓▓▓▓░░░░░░░░░ 50%                                              │ │
│ │ Ejecutando 6/12…                                                    │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│ Progreso                                                                │
│ ┌ TextGrid (mono, read-only; OK rows green, FAILED/ERROR rows red) ───┐ │
│ │ Modo: DRY-RUN (no conecta a nada)                                   │ │
│ │ [1/12] 10.10.1.21 ... OK (12ms)                                     │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

860×680 default, centred. Custom Fyne theme (`scTheme`) sets only
`ColorNamePrimary` / `ColorNameForegroundOnPrimary` to the web blue so the
high-importance *Ejecutar* matches the landing; everything else is stock
Fyne. Copy is **Spanish** — the desktop tool is used by field
colleagues; the web (ecosystem convention) is English. Keep that split; do
not half-translate either side.

## Export a SecureCRT script (shipped in v0.2.0)

The **third action** in the button row, after Run and before Open folder,
enabled as soon as both files validate (it does not need a run).

### What it is (decided 2026-09-17)

Some environments only reach the devices from a workstation where
**SecureCRT** is the sanctioned client and SmartConfigure itself cannot be
run. SecureCRT has a real scripting API (VBScript on Windows, Python on
all platforms; *Script → Run…*), so SmartConfigure exports **a script that
SecureCRT executes**: it connects to each device over SSH2, sends the
rendered template line by line, keeps a per-device log, and moves on.
SmartConfigure only writes the file — it never connects, never executes.

Verified API (VanDyke docs + reference scripts):

```vbscript
#$language = "VBScript"
#$interface = "1.0"
crt.Session.Connect "/SSH2 /L " & user & " /PASSWORD " & pass & " /P 22 /ACCEPTHOSTKEYS " & host
crt.Screen.Synchronous = True
crt.Session.LogFileName = logPath : crt.Session.Log True
crt.Screen.Send line & vbCr
crt.Screen.WaitForStrings ">", "]", timeoutSeconds      ' returns 0 on timeout
crt.Sleep milliseconds
crt.Session.Disconnect
```

### Behaviour contract (implemented in `internal/securecrt/`)

- **Inputs** go through the **same** `template.Load` / `excelsheet.Load`
  as Run, so a missing column fails here too, with the same error text in
  the progress console.
- **Rendering** uses `template.Render` per device (same as dry-run). Any
  unresolved `{VARIABLE}` aborts the whole export with the dry-run message
  (`line N has a variable with no value…`) — a script that sends a literal
  `{GATEWAY_IP}` is worse than no script.
- **Output**, inside `smartconfigure-output/` next to the Excel:
  - `smartconfigure-securecrt.vbs` — the batch as **one** VBScript
    (Windows, works on every SecureCRT version; the audience).
  - `smartconfigure-securecrt.py` — the same batch as a Python script
    (`#$language = "Python3"`), for SecureCRT builds that bundle Python
    or for macOS/Linux. Same logic, same model; both are generated from
    one Go struct so they cannot drift.
  - Both scripts write their own logs next to themselves:
    `<IP>.securecrt.log` via `crt.Session.Log`, one per device, so the Log
    Viewer can be taught to read them later (they are raw terminal
    transcripts, not SmartConfigure sections).
- **Script behaviour** (mirrors `sshrunner` semantics as closely as
  SecureCRT allows):
  - one `Sub Main` / `def main` that iterates a literal device table
    (IP, user, password, port, rendered lines) embedded in the script;
  - per device: `Connect` with `/SSH2 /L /PASSWORD /P /ACCEPTHOSTKEYS`
    inside `On Error Resume Next` / `try` — a failed connect is logged
    to the SecureCRT log and the loop continues with the next device
    (never aborts the batch);
  - `Screen.Synchronous = True`; after each `Send line & vbCr`, wait with
    `WaitForStrings(">", "]", lineTimeout)` (VRP prompts end in `>` in
    user view and `]` in system view) — on timeout, keep going, exactly
    as the Go engine does after `--line-timeout`; a fixed `crt.Sleep` of
    `--idle-timeout` between lines gives the device the same breathing
    room;
  - blank template lines are already gone (`template.Load` drops them);
  - `Disconnect`, wait until `Session.Connected` is false, next device;
  - at the end, `crt.Dialog.MessageBox` with *N devices attempted — see
    the logs in <folder>*.
- **Passwords are embedded in the script** (that is the point: unattended
  batch). The progress console prints the same one-line warning the README
  gives for logs, and the script's header comment repeats it. Never
  upload or commit the exported script.
- **After export** the progress console prints the two paths and *Open
  folder* is enabled.
- **Not the primary action.** Run keeps the high-importance style; the
  export is secondary, like *Open folder*.
- **CLI mirror**: `--export-securecrt` (bool) writes the same two files
  into `--out` and exits without connecting; honours `--port`,
  `--idle-timeout`, `--line-timeout` so the script uses the same timings
  as a direct run. GUI and CLI must never drift — the generator lives in
  `internal/securecrt/` and both call it.
- **Golden-file tests** in `internal/securecrt/` (`testdata/*.golden`,
  `go test ./internal/securecrt -update` to regenerate): the exact bytes
  of both scripts for a two-device fixture whose password contains `"`,
  `&`, a backslash, a tab and `ö`. VBScript strings escape `"` as `""` and
  splice non-ASCII as `ChrW(n)`; Python strings `\u`-escape.
- **Line endings** of the exported files: CRLF (SecureCRT on Windows
  opens them in the Script editor; Notepad-friendly).

### Later, only if needed

- A `.txt` per device with the bare rendered commands, for pasting by
  hand into any terminal (SecureCRT's *Paste* honours a per-line delay).
  Trivial once the generator exists; add it as a checkbox on the export,
  not a fourth button.
- Teaching the Log Viewer to read the SecureCRT logs.

## Rules

- **One primary action: Run** (`widget.HighImportance`, blue). Export is
  `MediumImportance`, Open folder `LowImportance`. Every button has an icon
  from Fyne's theme set — no emoji in labels.
- **Validate on pick, not on Run.** As soon as both files are chosen the
  window parses them and shows `✓ N equipo(s) · N línea(s) · N variable(s)`,
  or `✗ Template: …` / `✗ Excel: …` with the loader's own error. Run and
  Export stay disabled until it passes. The summary is in the regular face,
  not mono: Fyne's bundled monospace clips underscores in labels, and
  variable names are full of them (the console TextGrid is unaffected).
- **File labels read `name — folder`** so ellipsis truncation eats the
  folder tail, never the file name.
- **An empty Excel cell is an unresolved variable** (`template.Render`
  keeps the placeholder for `""`), so dry-run, run and export all flag it
  with the same *line N has a variable with no value* message.
- **Tests**: `internal/gui/gui_test.go` drives validate/run/export through
  Fyne's test driver (`test.NewApp()`), no display needed; set
  `SC_GUI_CAPTURE=<png>` to save a software-rendered capture. The GL
  driver cannot run under `go test`; for a real-window look, build and
  launch the binary.
- **Progress bar + status line** during a run (`Ejecutando 6/12…`), then a
  one-line summary (`Hecho: 10 OK, 2 fallidos.`; dry-run says *con
  variables sin valor* instead of *fallidos*).
- **Console is a TextGrid**, never a disabled Entry (grey text, wrong
  font). OK rows green, FAILED/ERROR/AVISO rows red — colour is
  reinforcement, the word is always there.
- **Long paths truncate with an ellipsis**, they never widen the window.
- **Dry-run ticked on open.** Never change the default.
- **Progress is a console, not a log viewer.** One line per device,
  `[i/N] IP ... OK (dur)` / `... FAILED (dur): error`; the full transcript
  is in the `.log`. Do not render sections in the window.
- **Errors are sentences in the console** (`ERROR: no se pudo leer el
  Excel: …`), and a dialog only when the user pressed Run with a file
  missing.
- **The output folder is next to the Excel** (`<excel dir>/smartconfigure-output`).
  Every export goes there too.
- **Window title carries the version** (`SmartConfigure v0.1.0`), so
  screenshots in bug reports identify the build.
