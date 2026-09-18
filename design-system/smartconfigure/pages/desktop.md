# Page: Desktop window (Fyne) — overrides MASTER

`internal/gui/gui.go` (`Launch`). This is a native Fyne window, so the web
tokens in MASTER §2–§4 **do not apply literally** — Fyne draws with its own
theme. What carries over is the *structure* and the *semantics*: one
primary action, dry-run first, status as words, literal text in mono.

## Purpose

The 90% path: bring a template, let the app generate the Excel, fill it,
keep Dry-run ticked, press Run, read the progress, open the folder. And the
10% path: export a SecureCRT script that runs the same batch from a
workstation where only SecureCRT may touch the devices.

**The user only has the template.** The Excel is an artefact of the
template (its columns *are* the template's variables), so the app produces
it — the user never builds a header row by hand. Decided 2026-09-17 after
the v0.2 window asked for both files up front.

## Layout (v0.3.0)

```
┌ SmartConfigure v0.3.0 ────────────────────────────────────────────── ✕ ┐
│ [mark] SmartConfigure (bold)                                            │
│ Push one command template to every device listed in an Excel sheet …    │
│ ┌ 1. Template ────────────────────────────────────────────────────────┐ │
│ │ [📄 Choose template (.txt)…]  template.txt — C:\…\batch-sept         │ │
│ │ ✓ 44 lines · 5 variables: SWITCH_NAME, MGMT_IP, …                   │ │
│ │ [＋ Generate Excel]                                                  │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│ ┌ 2. Devices ─────────────────────────────────────────────────────────┐ │
│ │ [☰ Choose Excel (.xlsx)…]     devices.xlsx — C:\…\batch-sept         │ │
│ │ ✓ devices.xlsx created next to the template with the header row …   │ │
│ │ [📄 Open in Excel]                                                   │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│ ┌ 3. Run ─────────────────────────────────────────────────────────────┐ │
│ │ ☑ Dry-run (validate only — nothing is sent to any device)           │ │
│ │ [▶ Run]  [⇩ Export SecureCRT script]  [📂 Open output folder]        │ │
│ │ ▓▓▓▓▓▓▓▓▓░░░░░░░░░ 50%                                              │ │
│ │ Running 6/12…                                                       │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
│ Progress                                                                │
│ ┌ TextGrid (mono, read-only; OK rows green, FAILED/ERROR rows red) ───┐ │
│ │ Mode: DRY-RUN (no connection)                                       │ │
│ │ [1/12] 10.10.1.21 ... OK (12ms)                                     │ │
│ └─────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
```

880×720 default, centred. Custom Fyne theme (`scTheme`) sets only
`ColorNamePrimary` / `ColorNameForegroundOnPrimary` to the web blue so the
high-importance *Run* matches the landing; everything else is stock Fyne.
Copy is **English** everywhere (window, console, dialogs, generated
Excel), like the web and every other app — decided 2026-09-17, replacing
the earlier Spanish-window rule. The README stays Spanish.

### Flow

1. **Choose template** → parsed on pick: `✓ N lines · N variables: …` or
   `✗ Template: …`. Enables *Generate Excel*.
2. **Generate Excel** → `excelsheet.Generate` writes
   `excelsheet.DefaultExcelPath(template)` — `devices.xlsx` next to the
   template, `devices-2.xlsx`… if taken, **never overwriting** a sheet
   someone may have filled. Sheet *Devices*: bold, tinted, frozen header
   `IP Address · Username · Password · <VAR>…`, width 22; sheet
   *Instructions* with the rules; **no sample row** (a made-up IP could end
   up in a real batch). The file becomes the chosen Excel, *Open in Excel*
   opens it, and the Devices note explains: fill one row per device, save,
   then Run. *Choose Excel (.xlsx)…* remains for an existing sheet.
3. **Run / Export** are enabled as soon as a template is parsed and an
   Excel path is set — even if the sheet is still empty, because the user
   fills it *after* choosing it. Both actions **re-read the Excel from
   disk** (`loadDevices`) and, on a loader error, write `✗ Excel: …` in the
   Devices note and *Nothing was run/exported: fix the Excel first.* in the
   status line, without touching the console. There is no cached batch.
4. CLI mirror: `--template t.txt --generate-excel <path|auto>` writes the
   same workbook and exits; `auto` = `DefaultExcelPath`.

## Export a SecureCRT script (shipped in v0.2.0)

The **second action** in the Run row, after Run and before Open output
folder, enabled together with Run (it does not need a run).

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
- **Validate on pick.** The template is parsed the moment it is chosen
  (`✓ N lines · N variables: …` / `✗ Template: …`); a chosen Excel is
  checked the same way (`✓ N devices` / `✗ Excel: …`) but only as
  information — Run and Export read it again. The notes are in the regular
  face, not mono: Fyne's bundled monospace clips underscores in labels, and
  variable names are full of them (the console TextGrid is unaffected).
- **Empty notes are hidden**, not blank rows (`setNote`). Fyne does not
  re-run a parent layout when a child is shown or hidden, so `setNote` and
  `showProgress` refresh the root container whenever visibility flips.
- **File labels read `name — folder`** so ellipsis truncation eats the
  folder tail, never the file name.
- **An empty Excel cell is an unresolved variable** (`template.Render`
  keeps the placeholder for `""`), so dry-run, run and export all flag it
  with the same *line N has a variable with no value* message.
- **Tests**: `internal/gui/gui_test.go` drives template → generate → fill
  → run/export through Fyne's test driver (`test.NewApp()`), no display
  needed; `internal/excelsheet/generate_test.go` proves `Load` accepts what
  `Generate` writes. Set
  `SC_GUI_CAPTURE=<png>` to save a software-rendered capture. The GL
  driver cannot run under `go test`; for a real-window look, build and
  launch the binary.
- **Progress bar + status line** during a run (`Running 6/12…`), then a
  one-line summary (`Done: 10 OK, 2 failed.`; dry-run says *with missing
  values* instead of *failed*). Export ends with *Script exported (N
  devices). Nothing was sent to any device.*
- **Console is a TextGrid**, never a disabled Entry (grey text, wrong
  font). OK rows green, FAILED/ERROR/WARNING rows red — colour is
  reinforcement, the word is always there.
- **Long paths truncate with an ellipsis**, they never widen the window.
- **Dry-run ticked on open.** Never change the default.
- **Progress is a console, not a log viewer.** One line per device,
  `[i/N] IP ... OK (dur)` / `... FAILED (dur): error`; the full transcript
  is in the `.log`. Do not render sections in the window.
- **Errors are sentences** in the console (`ERROR: could not write the
  script: …`) or in the section they belong to (`✗ Excel: …`); the only
  dialog is *could not create the Excel file* on a failed Generate.
- **Icon**: `assets/icon.svg` is the source; `Icon.png` (root, 512 px) is
  what `fyne-cross -icon` bakes into the `.exe`, and `internal/gui/icon.png`
  is embedded for `SetIcon` (window + taskbar) and the header mark. A PNG on
  purpose — Fyne rasterises SVG icons per requested size and the thin web
  glyph turned to mush at 24 px; the bold mark (rounded blue tile, white
  38-unit stroke) is drawn for that size. Regenerate both PNGs together.
- **The output folder is next to the Excel** (`<excel dir>/smartconfigure-output`).
  Every export goes there too.
- **Logical size survives monitor hops.** Fyne (2.6–2.8.1) on Windows
  with mixed-DPI monitors re-scales the content when the window crosses to
  another monitor but does not reliably re-size the native window, so each
  150% → 100% → 150% round trip shrank it by the DPI ratio and pointer
  mapping went with it. `keepLogicalSizeAcrossMonitors` polls
  `Canvas().Scale()` every 250 ms and re-applies the last stable logical
  size through `Resize` when the scale flips (sub-2-unit wobble ignored so
  rounding never accumulates). Verified with scripted hops between a 150%
  and a 100% monitor: 880×727 stays pixel-exact. Keep it until upstream
  fixes it, then delete.
- **Window title carries the version** (`SmartConfigure v0.1.0`), so
  screenshots in bug reports identify the build.
