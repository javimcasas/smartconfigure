# Page: Log Viewer — overrides MASTER

`landing/logs.html`; logic in `landing/logs.js` (`parseLog`, `parseCsv`,
`render`, `buildDevice`). Deployed at
`https://smartconfigure.hubsmartmatrix.com/logs.html`.

## Purpose

Turn a folder of `smartconfigure-output/*.log` + `report.csv` into a
per-device breakdown you can read with a colleague: what was sent, what
the device answered, where it stopped. Everything is parsed in the browser
— the files never leave the machine (they contain passwords).

## Layout

```
┌ [■] SmartConfigure · Log Viewer                        ← Landing  [⇄] [☾] ┐  topbar
│                                                                            │
│  LOG VIEWER                                                                │  eyebrow
│  Read a run.                                                               │  h1
│  Drop the .log files SmartConfigure wrote (and report.csv) …               │  lead
│                                                                            │
│  ┌ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┐        │  .dropzone (dashed)
│  │   [↑]  Drag .log / .csv files here          [Choose files]     │        │   (is-over → yellow)
│  └ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ─ ┘        │
│                                                                            │
│  ┌ 12 devices │ 10 OK │ 2 FAILED │ 1m 42s total ┐    [Clear]               │  .run-strip (aria-live)
│                                                                            │
│  ── Batch report (report.csv) ─────────────────────────────────────────    │  h2 (only if CSV loaded)
│  Device · Status · Duration · Error                                        │  table.spec-table
│                                                                            │
│  ── Devices ───────────────────────── (All) (OK) (Failed)   ───────────    │  h2 + .chips filter
│  ▸ ● 10.10.1.21   admin · port 22 · 44 lines · 8.2s            [OK]       │  <details class="device">
│  ▾ ● 10.10.1.22   admin · port 22 · 44 lines · 15.1s       [FAILED]       │
│      ┌ 10:32:01.118  [3/44]  sysname SW-CORE-02 ─────────────────┐         │  .tsec
│      │ sysname SW-CORE-02                                        │         │
│      └───────────────────────────────────────────────────────────┘         │
│      ┌ 10:32:16.240  STOPPED ────────────────────────────────────┐ (.err)  │
│      │ % Unrecognized command found at '^' position.             │         │
│      └───────────────────────────────────────────────────────────┘         │
│      Started 2026-08-31 10:31:58  ·  Finished 10:32:16                     │  .device-foot
└────────────────────────────────────────────────────────────────────────────┘
```

Mobile: run strip becomes a 2×2 grid; device summary wraps meta under the
IP; transcript `pre` keeps horizontal scroll.

## Rules

- **The parser is a mirror of `sshrunner.go`.** `parseLog` recognises: the
  title line (`DRY RUN` → dry run), `Key: value` header lines up to the
  first separator, sections as `separator / HH:MM:SS.mmm title / separator
  / body`, and the footer starting `RESULT:`. A section is an error if its
  title is `STOPPED`, contains `FAILED`, or its body matches the VRP error
  strings (`% Unrecognized command`, `% Wrong parameter`, `Error: `,
  `% Invalid`, `Unrecognized command found`). Keep this list in sync with
  `sshrunner`'s error regex.
- **Badges are words.** `OK` (green) / `FAILED` (red) / `DRY-RUN OK`
  (neutral) / `DRY-RUN FAILED` (red) / `INCOMPLETE` (neutral, no footer
  found — the run was killed or the log is still being written).
- **Dry-run is never green.** The run strip counts a dry-run OK under *OK*
  (the CSV does the same), but the badge stays neutral so the eye does not
  read "12 switches configured" off a dry run.
- **Run strip figures are mono, tabular.** Total time = sum of footer
  durations (formatted `1m 42s`, `8.2s`, `640ms`).
- **Device rows are `<details>`.** Native disclosure: keyboard, no JS
  state. Failed devices are rendered **open** by default when there are
  ≤ 5 of them; OK devices closed. *Expand all / Collapse all* is a ghost
  button next to the filter chips.
- **Filter chips** (`All · OK · Failed`) are radio inputs styled as chips;
  filtering hides rows with `hidden`, it never re-parses. The count in each
  chip is mono.
- **Sort** is the order files were dropped, then by IP when a CSV is
  present (the CSV order is the batch order). No sort UI.
- **Error section styling**: `.tsec.err` has a `--danger-text` left rule
  and its title is danger-coloured; the body stays plain mono. Only the
  first error section in a device is scrolled into view when the row opens.
- **Transcript bodies never wrap** (`white-space: pre`, horizontal scroll)
  — a wrapped CLI line lies about what was sent. `max-height: 280px`,
  vertical scroll inside.
- **Dropping**: the whole page is a drop target while dragging
  (`window` `dragover`/`drop` prevented so the browser never navigates to
  a file); the drop zone shows `.is-over`. `.csv` → `parseCsv`; anything
  else → `parseLog`; a file that does not parse (no `Device:` header, no
  sections) is listed in a `role="alert"` line under the drop zone:
  *2 files skipped: notes.txt, x.log — not SmartConfigure logs*. Never
  silently ignored.
- **Loading more files appends**; *Clear* resets everything and focuses
  the drop zone's button. Dropping a second `report.csv` replaces the
  first (the batch report is one file).
- **Nothing is stored** — no `localStorage` of log content, ever.

## Copy

- Eyebrow: *Log viewer*; H1: *Read a run.*
- Lead: *Drop the `.log` files SmartConfigure writes for each device, and
  `report.csv` if you have it. Parsed in this browser — nothing is
  uploaded, so it is fine that the logs contain what was sent.*
- Drop zone: *Drag `.log` and `.csv` files here* · button *Choose files*
- Empty state under the zone: *No files loaded yet.*
- Run strip labels: *devices · OK · failed · total*
- Section titles: *Batch report (report.csv)* / *Devices*
- Footer of a device without footer: *Log ended before the RESULT block.*
