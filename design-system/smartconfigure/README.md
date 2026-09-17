# SmartConfigure — design system

The style skill for this repo. Read before designing, building, reviewing
or restyling any surface of the app — the web landing, the Log Viewer, or
the Fyne desktop window.

## Files

| File | Use |
|------|-----|
| [`MASTER.md`](./MASTER.md) | Global source of truth — chosen style, colour/type/spacing tokens, components, a11y floor, anti-patterns. **Always read first.** |
| [`pages/landing.md`](./pages/landing.md) | `public/index.html` — downloads, how it works, input format, CLI flags |
| [`pages/logs.md`](./pages/logs.md) | `public/logs.html` — browser-side viewer for `*.log` + `report.csv` |
| [`pages/desktop.md`](./pages/desktop.md) | `internal/gui/gui.go` — the Fyne window (template -> Generate Excel -> fill -> Run), and the *Export SecureCRT script* contract |

Page files **override** `MASTER.md` on conflict.

## Chosen style (summary)

**"Field Manual"** = **Minimalism & Swiss Style** (flat panels, 1px rules,
numbered steps, spec tables, one primary action) + one **signal-blue**
accent (`#1D4ED8` fill/ink light · `#60A5FA` fill / `#93C5FD` ink dark)
that is spent only on the live/primary action.

Typography is a two-role split: **Manrope** for UI chrome, **JetBrains
Mono** for anything that is literally a template, a column name, a flag, a
file name or a transcript. Shared with HyperParse on purpose: console text
looks the same across the tools that touch consoles.

Why this blue: the user's explicit preference (a signal-yellow draft was
rejected). It is a deep blue-700, kept deliberately apart from the hub's
indigo (`#4f46e5`) and OceanStor's sky (`#0369a1`) so the three still read
differently side by side. Rationale and rejected alternatives in
`MASTER.md` §1.

## The one rule to remember

**Blue means live, mono means literal.** The accent goes on the thing
that does something real; anything a user will paste into a file or read
back from a device is set in mono, unwrapped, unformatted.

## Retrieval prompt

> I am working on the SmartConfigure **<landing | logs | desktop>**
> surface. Read `design-system/smartconfigure/MASTER.md`, then
> `design-system/smartconfigure/pages/<surface>.md`. Apply the page file
> where it conflicts with MASTER; otherwise apply MASTER. Use the
> tokens/classes in `public/style.css` as-is.

## Maintenance

- `public/style.css` `:root` is the real token store. When it changes,
  update `MASTER.md` §2 / §4 to match.
- Contrast pairs in `MASTER.md` §2 are **measured**, not estimated.
  Re-measure before changing any colour token.
- `public/index.html` download URLs must match the asset names in
  `.github/workflows/release.yml`. Change both in the same commit.
- `public/logs.js` `parseLog` mirrors `internal/sshrunner` writers.
  Change both in the same commit.
- Generated with the `ui-ux-pro-max` skill
  (`--design-system --variance 3 --motion 2 --density 6`), then curated to
  the codebase. Re-run the skill for fresh options; do not let it overwrite
  these curated files.
- `src/index.js` + `public/` are what Workers Builds deploys on every push
  to `main`. `design-system/`, `internal/` and `test` data never ship.
