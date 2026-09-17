# Page: Landing — overrides MASTER

`public/index.html`; behaviour in `public/theme.js` (theme, OS detection,
latest-release badge). Served by `src/index.js` behind SmartMatrix SSO at
`https://smartconfigure.hubsmartmatrix.com/` (visitors arrive through the
hub's `/launch/smartconfigure`).

## Purpose

Three jobs, in this order of importance:

1. **Hand out the right binary in one click** — the visitor's OS is
   detected and promoted to the hero button; the full list is one scroll
   away.
2. **Show the exact input shape** (template syntax, Excel columns) so a
   first-time user can build the files without opening the README.
3. **Set expectations honestly**: this runs on your machine, over SSH, on
   real gear; dry-run first.

The hub card already got the visitor here (the Worker only serves the page
to a valid SmartMatrix session); the page does not need to sell anything.

## Layout

```
┌ [■] SmartConfigure            How it works · Format · CLI · Log viewer  [⇄] [☾] ┐  topbar (sticky)
│                                                                                 │
│  DESKTOP TOOL · WINDOWS / MACOS / LINUX                                         │  eyebrow
│  Push one config template to every switch in an Excel.                          │  h1 (balanced, ≤ 2 lines)
│  Opens an SSH session per row, substitutes {VARIABLE}s, keeps a transcript…     │  lead (muted, 68ch)
│  [⬇ Download for Windows]  [All downloads]      v0.1.0 · 31 Aug 2026            │  primary + secondary + version tag
│                                                                                 │
│  ┃ Safe by default. Dry-run is on when the app opens: …                         │  .note (single accent left rule)
│                                                                                 │
│  ── Downloads ──────────────────────────────────────────────────────────────    │  h2#downloads
│  win  Windows x64        smartconfigure-windows-amd64.zip   unzip, run   [Get]  │  table.downloads
│  mac  macOS Apple Silicon smartconfigure-darwin-arm64       see note     [Get]  │   (.is-current on visitor's row)
│  mac  macOS Intel        smartconfigure-darwin-amd64        see note     [Get]  │
│  lin  Linux x64          smartconfigure-linux-amd64.tar.xz  tar -xJf     [Get]  │
│  Release history on GitHub →                                                    │
│                                                                                 │
│  ── How it works ───────────────────────────────────────────────────────────    │  h2#how
│  ① Write a template …   ┌ example/template.txt excerpt (pre) ┐                  │  ol.steps + code
│  ② Generate the Excel … ┌ header row table (mono) ┐                             │
│  ③ Run …                                                                        │
│  ④ Or export a SecureCRT script …                                               │
│                                                                                 │
│  ── What you get ───────────────────────────────────────────────────────────    │  h2#outputs
│  [log] Per-device log   [csv] report.csv   [check] Dry-run                      │  .outputs (3-up)
│                                                                                 │
│  ── Format ─────────────────────────────────────────────────────────────────    │  h2#format
│  Excel columns table · template rules list                                      │
│                                                                                 │
│  ── Command line ───────────────────────────────────────────────────────────    │  h2#cli
│  pre: smartconfigure --template … --excel … --dry-run                           │
│  flags table (mono flag · default · description)                                │
│                                                                                 │
│  ── Analyze a run ──────────────────────────────────────────────────────────    │  h2#viewer
│  ┌ Drop the .log files and report.csv into the Log Viewer … [Open Log Viewer] ┐ │  .note-like panel, secondary btn
│                                                                                 │
│  README · Releases · Part of SmartMatrix                                        │  footer
└─────────────────────────────────────────────────────────────────────────────────┘
```

Mobile (<720px): nav links hide except *Log viewer*; hero buttons stack
full-width; the downloads table becomes stacked rows (`display: grid` per
row: tag+platform / file / button) — no horizontal scroll; outputs 1-up;
flags table keeps its `.table-wrap` scroll (it is literal text).

## Rules

- **The primary button is the visitor's OS.** `theme.js` reads
  `navigator.userAgentData?.platform` then `navigator.platform` and picks
  `windows` / `mac` (arm64 preferred — Intel is the secondary link in the
  table; Apple Silicon is what a new Mac is) / `linux`; unknown → Windows
  (the audience). The matching `tr.downloads` row gets `.is-current`. No
  network needed for this.
- **Asset names are the workflow's.** Every `href` under `/releases/latest/download/`
  must be one of: `smartconfigure-windows-amd64.zip`,
  `smartconfigure-linux-amd64.tar.xz`, `smartconfigure-darwin-arm64`,
  `smartconfigure-darwin-amd64`. Change `release.yml` and this page
  together.
- **Version tag** is optional decoration: `theme.js` fetches
  `https://api.github.com/repos/javimcasas/smartconfigure/releases/latest`
  (public, CORS-enabled, 60 req/h unauthenticated); on success it fills
  `.version-tag` with `tag_name · published_at` and unhides it; on any
  error the tag stays hidden and nothing else changes. Cache the JSON in
  `sessionStorage` (`smartconfigure-release`) to stay well under the rate
  limit.
- **macOS note** is one sentence next to the two Mac rows: the binary is
  not notarised — first launch is right-click → *Open*, and
  `chmod +x` after download. True until the workflow signs builds.
- **The template excerpt is real.** Copy the first ~10 lines of
  `example/template.txt` verbatim into the `<pre>`; when the example file
  changes, update the excerpt.
- **Excel header table** shows the literal header row the app generates
  for the example template (`excelsheet.Generate`, `fixedHeader`):
  `IP Address · Username · Password · SWITCH_NAME · SSH_PASSWORD · GATEWAY_IP · MGMT_IP · SUBNET_MASK`,
  all in mono. The step says the app writes this file next to the template
  (*Generate Excel*) and the user fills one row per device; an existing
  sheet with the same shape also works. If `fixedHeader` changes, this
  table and `pages/desktop.md` change with it.
- **Flags table** mirrors `main.go`'s `flag.*` definitions: `--template`,
  `--excel`, `--out`, `--generate-excel`, `--dry-run`, `--export-securecrt`,
  `--port`, `--connect-timeout`, `--idle-timeout`, `--line-timeout`,
  `--version` with their defaults. Defaults are mono.
- **No feature that has not shipped.** SecureCRT export (v0.2.0) and
  Generate Excel (v0.3.0) are on the page because the binaries are on
  Releases; the next feature waits for its tag.
- **Security line** (footer or Format section): logs contain what was sent,
  including passwords if the template sends them; treat
  `smartconfigure-output/` as sensitive. One sentence, no scare styling.
- **Theme toggle** in the topbar; the page honours the system theme on
  first visit. No flash: `theme.js` is loaded in `<head>` (tiny, sync)
  before the stylesheet paints.
- **App switcher**: `<div data-smartmatrix-switcher data-current="smartconfigure"></div>`
  in `.topbar-actions` + `https://hubsmartmatrix.com/switcher.js` deferred,
  exactly like every SSO app — the switcher links through `/launch/<id>` so
  the hub still owns access.

## Copy

- Eyebrow: *Desktop tool · Windows / macOS / Linux*
- H1: *Push one config template to every switch in an Excel.*
- Lead: *SmartConfigure opens an SSH session to each device listed in your
  spreadsheet, sends the template with its `{VARIABLES}` filled from that
  row, and keeps a full transcript per device. It runs on your machine —
  nothing goes through the hub.*
- Note: *Safe by default. Dry-run is on when the app opens: it renders every
  line for every device and flags missing values without connecting to
  anything. Untick it when the log looks right.*
- Steps: *Write a template* / *Generate the Excel, then fill it* / *Run —
  dry first, then live* / *Or export a SecureCRT script*.
- Outputs: *Per-device log* — *A readable transcript of the whole SSH
  conversation, `<IP>.log`.* / *report.csv* — *One row per device: OK or
  FAILED, duration, error.* / *Dry-run* — *Same log format, nothing sent.*
- Viewer panel: *Already ran a batch? Drop the `.log` files and `report.csv`
  into the Log Viewer for a per-device breakdown. Parsed in your browser,
  nothing is uploaded.*
- Buttons: *Download for Windows* / *Download for macOS* / *Download for
  Linux*; table rows: *Download*; *All downloads* (anchor to `#downloads`);
  *Open Log Viewer*.
- Tone: English, second person, short sentences, no exclamation marks.
