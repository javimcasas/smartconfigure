# Design System: SmartConfigure — MASTER (Source of Truth)

> Global design rules for **SmartConfigure**, the SmartMatrix bulk-SSH
> configuration tool. Surface-specific deviations live in `pages/*.md` and
> **override** this file. Retrieval: read this file first; then
> `pages/<surface>.md`.

Generated with the `ui-ux-pro-max` skill (`--design-system` on *"internal
network engineering desktop tool landing page download binaries SSH bulk
configuration log viewer"*, `--variance 3 --motion 2 --density 6`), then
adapted by hand: the skill's pattern match ("event landing", countdowns) was
discarded, its style match (Minimalism & Swiss) kept, its palette replaced
by the ecosystem colour map below.

---

## 0. What this is

SmartConfigure is **not a web app**. It is a Go desktop program (Fyne GUI +
CLI) that opens SSH sessions to many network devices and sends the same
command template to each one, with `{VARIABLE}` placeholders filled from an
Excel row. The web part is a **static landing** on Cloudflare (assets only,
no Worker, no SSO) that exists to hand out the binaries, explain the input
format, and host a browser-side **Log Viewer** for the files the tool
produces.

| File | Role |
|------|------|
| `main.go`, `internal/*` | The desktop tool. `gui/gui.go` is the Fyne window; `batch/` drives one run; `sshrunner/` writes the per-device `.log`; `report/` writes `report.csv` |
| `landing/index.html` | Landing: downloads, how it works, input format, outputs, CLI flags |
| `landing/logs.html` + `landing/logs.js` | Log Viewer: drop `.log` / `report.csv`, parsed in the browser |
| `landing/style.css` | **The entire web design system** — tokens in `:root`, every component class |
| `landing/theme.js` | Theme init + toggle, OS detection for the primary download button, latest-release badge |
| `landing/favicon.svg` | Brand mark |
| `wrangler.jsonc` | Cloudflare config: `assets.directory = "landing"`, nothing else. No Worker script. |
| `.github/workflows/release.yml` | Tag `vX.Y.Z` → binaries on GitHub Releases. **The landing's download URLs must match the asset names this workflow produces.** |

### The three surfaces

| Surface | Where | Page doc |
|---------|-------|----------|
| **Landing** | `landing/index.html` | `pages/landing.md` |
| **Log Viewer** | `landing/logs.html` | `pages/logs.md` |
| **Desktop** (Fyne window) | `internal/gui/gui.go` | `pages/desktop.md` |

### Who uses it and how

**A network engineer commissioning a batch of switches** (typically Huawei
VRP gear: `system-view`, `sysname`, `commit`). They have a template of the
config block and an Excel with one row per device. They will:

1. Download the binary once (Windows, mostly), unzip, double-click.
2. Pick the template and the Excel, keep **Dry-run** on, run, read the log.
3. Untick Dry-run, run for real, open the output folder.
4. Maybe drop the logs into the Log Viewer to review a run with a colleague.

Consequences that outrank everything else:

1. **The tool touches production hardware.** Every surface must make the
   *dry-run first, live second* path the obvious one. The accent colour is
   spent on the live/primary action so it reads as "this one does something".
2. **The inputs are files with an exact shape.** The landing must show the
   template syntax and the Excel column order as literal text, in mono, so
   people can copy the shape. Never paraphrase a format.
3. **Nothing is uploaded.** The Log Viewer parses locally; the landing says
   so. The download page must never look like a SaaS sign-up.
4. **Some environments only allow a jump host / remote desktop.** The
   planned *Export SecureRDP script* button (see `pages/desktop.md`) exists
   for those; the web copy should not promise it until it ships.

It is a technical datasheet, not a marketing site: no testimonials, no
countdowns, no pricing, no hero illustration.

### The data the web renders

- **Releases**: GitHub Releases of `javimcasas/smartconfigure`. Assets, as
  produced by `release.yml` since the CI rename commit:
  `smartconfigure-windows-amd64.zip`, `smartconfigure-linux-amd64.tar.xz`,
  `smartconfigure-darwin-arm64`, `smartconfigure-darwin-amd64`. Stable URL:
  `https://github.com/javimcasas/smartconfigure/releases/latest/download/<asset>`.
  (`v0.1.0` still carries the old unsuffixed `smartconfigure.zip` /
  `smartconfigure.tar.xz`; the next tag fixes that.)
- **Per-device log** (`sshrunner.writeHeader/writeSection/writeFooter`):
  a title line (`SmartConfigure session log` or `… DRY RUN`), `Key: value`
  header lines, then repeated `separator / HH:MM:SS.mmm TITLE / separator /
  body` sections, then `separator / RESULT: … / Finished / Duration /
  separator`. The Log Viewer's parser mirrors this **exactly** — change the
  Go writer and the JS parser together.
- **report.csv**: `Device,Status,Duration,Error`; `Status` is `OK` or
  `FAILED`; `Error` may contain commas (join the tail).

---

## 1. Chosen style — "Field Manual"

### Primary: **Minimalism & Swiss Style** (`minimalism-and-swiss-style`)
The skill's top match for *enterprise apps, documentation sites,
professional tools* (`accessibility: risk:low`, `performance: cost:low`).
What we take:

- **Flat surfaces.** Panels are `--surface` + a 1px `--border`. **No resting
  shadows.** Nothing floats on the landing; the only elevated thing in the
  whole web is the Log Viewer's drop veil.
- **Grid discipline.** One column of prose at a readable measure; multi-column
  only for the download list and the outputs trio, and both collapse to one
  column on phones.
- **Numbered, literal, tabular.** Steps are numbered; formats are shown as
  code; flags are a table. The page reads like a field manual's quick-start
  card.
- **One primary action per surface** — *Download for <your OS>* on the
  landing, *Choose files* on the Log Viewer, *Run* on the desktop.

### Density: `--density 6/10` (standard)
Section spacing 48–64px, panel padding 20–24px, 8px rhythm. The landing is
read once; the Log Viewer is denser (see its page file: 36px rows).

### Signature moves

1. **Yellow means "live".** One accent. It is spent on: the primary button,
   the brand tile, focus rings, the active nav/filter, the *live* badge on
   the desktop Run button. It is **never** a background wash, never
   decorative, never a heading colour. The message: yellow is the thing that
   touches a real switch.
2. **Mono means literal.** Template text, `{VARIABLE}` names, Excel column
   headers, file names, CLI flags, log transcripts, IPs, durations — all in
   **JetBrains Mono**, unformatted. Everything else in **Manrope**. Never a
   label in mono, never a value in the UI face.
3. **Dry-run is the default, and the copy says so everywhere.** The landing's
   "Safe by default" note, the desktop's pre-ticked checkbox, the Log
   Viewer's `DRY-RUN OK` badge in neutral (not green) so a dry run is never
   mistaken for a real success.
4. **The download block is a table, not cards.** Platform · file · note ·
   button, one row each; the row for the visitor's OS is promoted to the
   hero button. No OS logos: a 2-letter mono tag (`win`, `mac`, `lin`) is
   enough and never goes out of date.
5. **Status is words.** `OK`, `FAILED`, `DRY-RUN OK`, `INCOMPLETE` are text
   badges; colour is reinforcement.

### Rejected (do not use)

- **Event / marketing landing patterns** (skill default: countdown, social
  proof, sticky register CTA). This is an internal tool with one download.
- **Navy + gold** (skill palette). Navy collides with the hub's indigo and
  OceanStor's blue side by side; gold-as-brown collides with SmartFinance's
  brass.
- **Ink as the action colour.** SmartFinance owns it ("Ledger").
- **OS logos** (Windows flag, Apple, Tux): brand-usage headaches, and they
  date. Text tags instead.
- **Cards with shadows, gradients, glass, hero illustrations, emoji icons.**
- **Green for dry-run.** A dry run that "succeeded" proved the inputs, not
  the devices. Neutral badge.
- **Native `confirm()` / `alert()`** on the web (the desktop uses Fyne's
  dialogs, which is fine there).

---

## 2. Color tokens — SOURCE OF TRUTH = `landing/style.css` `:root`

Do not introduce a second accent hue, a gradient, or a raw hex in HTML or
JS. Add a token here first. Every token exists in **both** themes.

### Light (`:root`)

| Role | Token | Value |
|------|-------|-------|
| Page background | `--bg` | `#F6F7F9` |
| Panel / surface | `--surface` | `#FFFFFF` |
| Recessed fill (code blocks, table header, drop zone) | `--surface-2` | `#EEF0F4` |
| Divider / panel outline | `--border` | `#DDE1E8` |
| **Control boundary** (inputs, secondary buttons) | `--field-border` | `#7A8598` |
| Text | `--text` | `#111827` |
| Text muted | `--text-muted` | `#4B5563` |
| **Accent fill** (primary button, brand tile, focus ring) | `--primary` | `#B47509` |
| Accent fill hover | `--primary-hover` | `#CA8A04` |
| **Accent as text / icon** (links, active nav) | `--primary-ink` | `#854D0E` |
| Text on accent fill | `--on-primary` | `#111827` |
| Accent tint (active filter, brand tile bg, drop veil) | `--primary-tint` | `#FEF3C7` |
| Success bg / text | `--success-bg` `--success-text` | `#E7F6EE` `#0F7A46` |
| Danger bg / text | `--danger-bg` `--danger-text` | `#FDECEC` `#B91C1C` |
| Neutral badge bg / text (dry-run, incomplete) | `--neutral-bg` `--neutral-text` | `#EEF0F4` `#4B5563` |
| Floating shadow (drop veil only) | `--shadow-float` | `0 8px 24px rgba(17,24,39,.14)` |

### Dark (`:root[data-theme="dark"]`)

| Role | Token | Value |
|------|-------|-------|
| Page background | `--bg` | `#0F1117` |
| Panel / surface | `--surface` | `#171A22` |
| Recessed fill | `--surface-2` | `#1F232D` |
| Divider | `--border` | `#2C3140` |
| Control boundary | `--field-border` | `#6B7486` |
| Text | `--text` | `#E8EAF0` |
| Text muted | `--text-muted` | `#9AA3B5` |
| Accent fill | `--primary` | `#FACC15` |
| Accent fill hover | `--primary-hover` | `#FDE047` |
| Accent as text | `--primary-ink` | `#FDE047` |
| Text on accent fill | `--on-primary` | `#1C1400` |
| Accent tint | `--primary-tint` | `#332A0C` |
| Success bg / text | | `#0E2A1C` `#3DD68C` |
| Danger bg / text | | `#2C1417` `#F58080` |
| Neutral badge bg / text | | `#1F232D` `#9AA3B5` |
| Floating shadow | `--shadow-float` | `0 8px 24px rgba(0,0,0,.5)` |

The hub's app switcher (`switcher.js`) reads `--surface/--text/--text-muted/
--border/--primary`; all five exist in both themes, so it themes itself.

### The two-token accent rule

`--primary` is a **fill** (always paired with `--on-primary`, which is dark
in both themes — yellow needs dark text). `--primary-ink` is **text** on a
surface. They are different values in both themes. Call sites must use the
right one; `--primary` as a text colour fails contrast in light mode.

### Verified contrast (measured, both themes)

| Pair | Light | Dark |
|------|------:|-----:|
| `--text` on `--surface` | 17.74 | 14.46 |
| `--text` on `--surface-2` | 15.55 | 13.06 |
| `--text-muted` on `--surface` | 7.56 | 6.86 |
| `--text-muted` on `--surface-2` | 6.62 | 6.20 |
| `--primary-ink` on `--surface` | 6.85 | 13.19 |
| `--primary-ink` on `--primary-tint` | 6.15 | 9.97 |
| `--on-primary` on `--primary` | 4.64 | 11.93 |
| `--on-primary` on `--primary-hover` | 6.04 | 13.86 |
| `--success-text` on `--success-bg` | 4.83 | 8.19 |
| `--danger-text` on `--danger-bg` | 5.66 | 6.79 |
| `--neutral-text` on `--neutral-bg` | 6.62 | 6.20 |
| `--primary` fill vs `--surface` (non-text, 3:1) | 3.60 | 11.36 |
| `--field-border` vs `--surface-2` (non-text, 3:1) | 3.27 | 3.34 |

**Tightest pairs:** `--on-primary`/`--primary` light (4.64) and
`--primary` fill vs white (3.60). Do not lighten the light-mode fill:
`#CA8A04` is the hover, and it already drops to 2.94 against white — it is
acceptable as a *hover* state only because the resting state establishes
the boundary. Re-measure any new pair before shipping (script: WCAG
relative luminance).

### Semantic colour discipline

- **Yellow = the action / the current thing / live.** Primary button, active
  nav link, active filter chip, focus ring, brand tile. Nothing else.
- **Green = a device really succeeded** (`OK`, `SUCCESS`). Never for a dry
  run.
- **Red = a device failed** (`FAILED`, error sections in a transcript),
  always with the word.
- **Neutral grey = informational status** (`DRY-RUN OK`, `INCOMPLETE`,
  version badge).
- There is **no amber warning tier** on the web: amber would read as the
  accent. If a warning state is ever needed, it is a neutral badge with the
  word *Warning*.

---

## 3. Typography

```html
<link href="https://fonts.googleapis.com/css2?family=Manrope:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500&display=swap" rel="stylesheet">
```

Two **semantic roles**, not heading/body:

| Face | Token | Used for | Never for |
|------|-------|----------|-----------|
| **Manrope** | `--font-ui` | h1–h3, prose, labels, buttons, nav, table headers, badges | anything the user will type into a file |
| **JetBrains Mono** | `--font-mono` (`.mono`, `code`, `pre`, `kbd`) | template snippets, `{VARIABLE}`, Excel headers, file/asset names, CLI flags, IPs, ports, durations, transcript sections, the version tag | headings, buttons, prose |

Scale (px): 12 (eyebrow, table header, badges) · 13 (hints, mono inline) ·
13.5 (mono blocks) · 14 (small buttons, meta) · 15 (buttons) · 16 (body) ·
18 (h3) · 22 (h2) · 34/28 (h1 desktop/mobile). Weights: 400 body, 500
labels/mono, 600 h2/h3/buttons, 700 h1 + wordmark. Numeric data uses
`font-variant-numeric: tabular-nums`.

Line length: prose measure `max-width: 68ch`. Headings `text-wrap: balance`.

**Mobile:** body stays 16px; every text control is ≥16px so iOS does not
zoom on focus.

---

## 4. Spacing, radius, elevation, motion

- **Spacing tokens** `--space-1…8` = 4 · 8 · 12 · 16 · 24 · 32 · 48 · 64 px.
  Section gap 64 (48 on mobile). Panel padding 24 (16 on mobile). Table
  cell padding 12×16.
- **Radius** `--radius` 6px everywhere (panels, buttons, code blocks);
  `--radius-sm` 4px (badges, kbd); pills `999px` only for status badges.
- **Elevation:** none. `--shadow-float` on the Log Viewer's drop veil only.
- **Motion** `--t-fast` 120ms (hover/press), `--t-base` 180ms (chevron,
  theme swap). One easing `cubic-bezier(.2,0,0,1)`. Buttons press 1px down.
  `prefers-reduced-motion` disables all transitions. No scroll-triggered
  reveals, no GSAP, no page transitions — the skill's motion snippet was
  not adopted.

---

## 5. Layout

- Single column, `max-width: 1040px` for the page, `68ch` for prose blocks,
  24px gutters (16px under 640px).
- Sticky **topbar** (surface, 1px bottom border, no blur): brand left,
  section nav + app switcher + theme toggle right. Nav collapses to
  *Log viewer* + toggle under 720px.
- `main > * { min-width: 0 }` so wide code blocks and tables scroll inside
  their wrappers instead of widening the page. **Never remove that rule** —
  it is what keeps 375px free of horizontal scroll.
- Tables (`.spec-table`) sit in a `.table-wrap` with `overflow-x: auto`.
- Code blocks (`pre`) scroll horizontally; they never wrap template text
  (a wrapped template would be a wrong template).

---

## 6. Components

| Component | Class | Notes |
|-----------|-------|-------|
| Topbar | `.topbar` | brand tile (yellow square, glyph) + wordmark; `.topbar-nav` anchors; `.topbar-actions` |
| Brand tile | `.brand-mark` | 32×32, `--primary` fill, `--on-primary` glyph (the three-node network glyph from `hub-icons.js`) |
| Eyebrow | `.eyebrow` | 12px uppercase, `letter-spacing: .06em`, muted |
| Buttons | `.btn` + `.btn-primary` / `.btn-secondary` / `.btn-ghost`, `.btn-sm` | 40px min height (32 sm); primary is the only filled one; secondary has `--field-border`; ghost has no border |
| Icon button | `.icon-btn` | 36×36, always `aria-label` (theme toggle) |
| Badge | `.badge` (+ `.ok` `.fail` `.neutral`) | 12px, 600, pill; always contains a word |
| Version tag | `.version-tag` | mono, neutral badge; populated by `theme.js` from the GitHub API, hidden until then |
| Section | `section.block` + `h2` | 64px apart; `h2` has an anchor `id` used by the topbar nav |
| Steps | `ol.steps > li` | numbered with a 28px circle (`--surface-2`, mono digit); title bold, body muted |
| Spec table | `table.spec-table` in `.table-wrap` | 12px uppercase headers, 16px cells, mono where literal; zebra-less; 1px rules |
| Download table | `table.downloads` | rows: OS tag (`.os-tag`, mono) · platform · file (mono) · note · `.btn-secondary`; the row matching the visitor's OS gets `.is-current` (yellow left rule) |
| Code block | `pre > code` | `--surface-2`, 13.5px mono, 16px padding, `overflow-x: auto`, `tab-size: 2` |
| Note | `.note` | `--surface` + 1px border + 3px **left** rule in `--primary`; used once per page at most (the safety note) |
| Outputs trio | `.outputs` | 3-up grid of `.output` (icon 20px stroke 1.8, h3, one sentence); 1-up on mobile |
| Footer | `.footer` | muted, mono for the repo path, links in `--primary-ink` |
| Drop zone (viewer) | `.dropzone` | dashed `--field-border`, `--surface-2`; `.is-over` swaps to `--primary` + tint |
| Run strip (viewer) | `.run-strip` | one bordered row of four cells (Devices · OK · Failed · Total time), mono figures |
| Device row (viewer) | `.device` (`<details>`) | summary: dot + IP (mono) + meta + badge + chevron; body: transcript sections |
| Transcript section (viewer) | `.tsec` (+ `.err`) | head: mono timestamp + title; body: `pre`, max-height 280px |
| Filter chips (viewer) | `.chips > .chip` | `role=radiogroup`; active chip = tint + ink + 1px `--primary` |

Icons: inline SVG, `viewBox="0 0 24 24"`, `stroke-width="1.8"`, round
caps/joins — the same stroke style as `SmartMatrix/public/hub-icons.js`.
Decorative icons next to text carry `aria-hidden="true"`.

---

## 7. Accessibility floor

- Skip link to `#main`; heading order h1 → h2 → h3; one `h1` per page.
- Every control has a visible label or an `aria-label` (theme toggle, file
  input, chevrons).
- Keyboard: all interactive elements are buttons/links/`summary`; the drop
  zone always has a *Choose files* button; filter chips are radio buttons
  under the hood.
- Focus ring: 2px `--primary`, offset 2px, on `:focus-visible` everywhere.
- Live regions: the Log Viewer's run strip is `aria-live="polite"`; parse
  errors use `role="alert"`.
- Contrast ≥ 4.5:1 for all text, ≥ 3:1 for control borders (measured §2).
- Motion respects `prefers-reduced-motion`.
- Theme honours `prefers-color-scheme` on first visit and remembers the
  toggle in `localStorage` (`smartconfigure-theme`; the old
  `smartmatrix_theme` key is read once as a fallback).

---

## 8. Anti-patterns (specific to this app)

- Download URLs that do not match `release.yml` asset names. When the
  workflow changes an asset name, change `index.html` in the same commit.
- Showing a feature on the landing before it ships (SecureRDP export).
- Rendering a template or column name in the UI face, or wrapping a
  template line.
- Colouring a dry run green.
- A second accent hue, an amber "warning" tier, gradients, glass, resting
  shadows, OS logos, emoji.
- Rewriting the log parser's structure without changing
  `sshrunner.go` in the same commit (they are one format).
- Adding a Worker or SSO gate: the landing is deliberately public and
  static (the binary runs where the hub cannot reach).
