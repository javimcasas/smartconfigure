# SmartConfigure — design system

The style skill for this repo. Read before designing, building, reviewing
or restyling any surface of the app — the web landing, the Log Viewer, or
the Fyne desktop window.

## Files

| File | Use |
|------|-----|
| [`MASTER.md`](./MASTER.md) | Global source of truth — chosen style, colour/type/spacing tokens, components, a11y floor, anti-patterns. **Always read first.** |
| [`pages/landing.md`](./pages/landing.md) | `landing/index.html` — downloads, how it works, input format, CLI flags |
| [`pages/logs.md`](./pages/logs.md) | `landing/logs.html` — browser-side viewer for `*.log` + `report.csv` |
| [`pages/desktop.md`](./pages/desktop.md) | `internal/gui/gui.go` — the Fyne window, and the contract for the planned *Export SecureRDP script* button |

Page files **override** `MASTER.md` on conflict.

## Chosen style (summary)

**"Field Manual"** = **Minimalism & Swiss Style** (flat panels, 1px rules,
numbered steps, spec tables, one primary action) + one **signal-yellow**
accent (`#B47509` fill / `#854D0E` ink light · `#FACC15` fill / `#FDE047`
ink dark) that is spent only on the live/primary action.

Typography is a two-role split: **Manrope** for UI chrome, **JetBrains
Mono** for anything that is literally a template, a column name, a flag, a
file name or a transcript. Shared with HyperParse on purpose: console text
looks the same across the tools that touch consoles.

Why yellow: every other hue is taken by a sibling app (hub indigo, OceanStor
blue, SmartProject teal, SmartNutrition green, SmartQuiz violet,
HyperTraining orange, HyperText vermilion, SmartFinance ink + brass,
HyperParse magenta), and yellow is the caution colour — this is the tool
that touches production switches, and its whole ethos is *dry-run first*.
Rationale and rejected alternatives in `MASTER.md` §1.

## The one rule to remember

**Yellow means live, mono means literal.** The accent goes on the thing
that does something real; anything a user will paste into a file or read
back from a device is set in mono, unwrapped, unformatted.

## Retrieval prompt

> I am working on the SmartConfigure **<landing | logs | desktop>**
> surface. Read `design-system/smartconfigure/MASTER.md`, then
> `design-system/smartconfigure/pages/<surface>.md`. Apply the page file
> where it conflicts with MASTER; otherwise apply MASTER. Use the
> tokens/classes in `landing/style.css` as-is.

## Maintenance

- `landing/style.css` `:root` is the real token store. When it changes,
  update `MASTER.md` §2 / §4 to match.
- Contrast pairs in `MASTER.md` §2 are **measured**, not estimated.
  Re-measure before changing any colour token.
- `landing/index.html` download URLs must match the asset names in
  `.github/workflows/release.yml`. Change both in the same commit.
- `landing/logs.js` `parseLog` mirrors `internal/sshrunner` writers.
  Change both in the same commit.
- Generated with the `ui-ux-pro-max` skill
  (`--design-system --variance 3 --motion 2 --density 6`), then curated to
  the codebase. Re-run the skill for fresh options; do not let it overwrite
  these curated files.
- Only `landing/` is deployed (Cloudflare assets). `design-system/` never
  ships.
