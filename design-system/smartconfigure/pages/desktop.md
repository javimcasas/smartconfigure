# Page: Desktop window (Fyne) — overrides MASTER

`internal/gui/gui.go` (`Launch`). This is a native Fyne window, so the web
tokens in MASTER §2–§4 **do not apply literally** — Fyne draws with its own
theme. What carries over is the *structure* and the *semantics*: one
primary action, dry-run first, status as words, literal text in mono.

## Purpose

The 90% path: pick two files, keep Dry-run ticked, press Run, read the
progress, open the folder. And the planned 10% path: produce a script to
run the same batch from an environment where only SecureRDP can reach the
devices.

## Layout (current, v0.1.x)

```
┌ SmartConfigure v0.1.0 ─────────────────────────────────────────── ✕ ┐
│ SmartConfigure (bold)                                                │
│ Ejecuta un template de comandos SSH sobre varios equipos …           │
│ ─────────────────────────────────────────────────────────────────── │
│ [Elegir template (.txt)...]                                          │
│ C:\…\template.txt                                                    │
│ [Elegir Excel (.xlsx)...]                                            │
│ C:\…\equipos.xlsx                                                    │
│ ☑ Dry-run (no conectar a los equipos, solo validar)                  │
│ [▶  Ejecutar]                 [📂 Abrir carpeta de resultados]       │
│ ─────────────────────────────────────────────────────────────────── │
│ Progreso:                                                            │
│ ┌ read-only multiline (console) ──────────────────────────────────┐ │
│ │ Modo: DRY-RUN (no conecta a nada)                                │ │
│ │ 12 equipo(s), 44 línea(s) de template                            │ │
│ │ [1/12] 10.10.1.21 ... OK (12ms)                                  │ │
│ └──────────────────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────────────────┘
```

720×600 default. Copy is **Spanish** — the desktop tool is used by field
colleagues; the web (ecosystem convention) is English. Keep that split; do
not half-translate either side.

## Layout (planned: SecureRDP export)

Add a **third action** to the button row, after Run and before Open folder,
enabled as soon as both files are chosen (it does not need a run):

```
│ ☑ Dry-run (no conectar a los equipos, solo validar)                  │
│ [▶  Ejecutar]   [⇩ Exportar script SecureRDP]   [📂 Abrir carpeta]   │
```

Behaviour contract (to implement in the next session):

- Reads the template + Excel through the **same** `template.Load` /
  `excelsheet.Load` as Run, so a missing column fails here too, with the
  same error text in the progress console.
- Renders every device's lines with `template.Render` and writes **one
  script file** (format: open question, see below) into
  `smartconfigure-output/`, next to the logs; the progress console prints
  the path and *Open folder* is enabled.
- Never connects. It is a pure file export: dry-run semantics, so the
  button is **not** the primary (Run keeps that role); style it like *Open
  folder* (secondary). If Fyne's high-importance style is used for Run,
  export is medium importance.
- Unresolved `{VARIABLE}`s abort the export with the same message dry-run
  uses (`line N has a variable with no value…`) — a script with a literal
  `{GATEWAY_IP}` in it is worse than no script.
- Passwords from the Excel end up in the script. Print the same one-line
  warning the README gives for logs, in the progress console, every time.
- Expose it on the CLI too: `--export-securerdp <path>` (mirrors GUI; the
  CLI and GUI must never drift, see `batch.Options`).

**Open question for the next session** — the script format itself. Before
writing any generator we need one real example of what SecureRDP executes
(a `.vbs`/`.py` per session like SecureCRT's scripting? a batch of
`connect host user pass` lines? one file per device or one for the batch?
how it sends a line and waits — prompt match or delay?). Get that sample
first; the generator is a `template`-style package (`internal/securerdp/`)
with a golden-file test against that sample.

## Rules

- **One primary action: Run.** Everything else is secondary.
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
