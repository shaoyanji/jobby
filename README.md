# jobby

A composable framework for job applications: one JSON Resume source, two
languages, and a sub-second PDF loop you can drive from a TUI, a flag, or your
own editor.

The CV is rendered by a small layout compiler (`internal/layout`) that turns the
semantic resume into pdfcpu's declarative `create` document: it measures text
with the same Helvetica metrics pdfcpu draws with, advances a y cursor, breaks
pages and emits explicit coordinates. pdfcpu is the only renderer, so a layout
question can always be answered by reading the JSON that produced the page.

## Why

A CV is data, but most tools make it a document. Once it is a document, every
application produces a new copy that drifts from the last one and you lose the
history of your own career arc.

jobby keeps the source in `resume.json` (JSON Resume schema plus a few custom
fields), keeps German translations in a content-addressed sidecar, and treats a
CV as a *view*: pick a loadout, pick a language, compile.

## Quick start

```bash
nix develop                          # go, pdfcpu 0.16, mupdf, translate-shell, task
task                                 # list the pipeline
task app:build COMPANY=TestGmbH LENS=ai LANG=de   # capsule -> CV + Anschreiben
task app                             # the TUI
task preview                         # the legacy resumectl/typst preview
```

pdfcpu must be **>= 0.16**: `create`, layout guides and the `contentBox` rail
arrived after the 0.11 the base image ships, which is why `flake.nix` pins the
renderer from its own nixpkgs revision. Without Nix, install `go`,
`pdfcpu>=0.16`, `mupdf`, `translate-shell` and `go-task` yourself.

There is no `nix` in a Codespace by default: `curl -L https://nixos.org/nix/install | sh -s -- --no-daemon`
followed by `. ~/.nix-profile/etc/profile.d/nix.sh` gives you the flake, and
`nix develop` then supplies every dependency at pinned versions.

## One folder per application (the workbench)

`cmd/workbench` builds an **application capsule** instead of a CV variant:

```
applications/2026-10-24-testgmbh/
  capsule.json                      lens, title overrides, bullets, letter, language
  rendered.json                     the exact CV that was compiled
  CV_Matt_Ji.pdf                    capsule + rendered JSON embedded
  Cover_Letter_Matt_Ji.pdf          same header, same stamp, same palette
  Application_Package_Matt_Ji.pdf   letter + CV + optional Zeugnisse
```

There is still **one** `resume.json` and **one** `i18n_cache.json`. An application
is a folder of decisions, not a branch. Because the state is embedded in the
PDFs, an old application is recoverable months later:

```bash
task app:import PDF=~/Downloads/CV_Matt_Ji.pdf   # restores capsule.json
```

### Narrative lenses

`lenses.json` holds three ways to tell the same career. A lens shifts the
persona line, rewrites per-company job titles, and pre-selects the bullets that
belong to that story:

| Lens | Persona | Bountystash title | Prioritises |
| :--- | :--- | :--- | :--- |
| `ai` | Software Engineer — AI systems, retrieval, evaluation | Senior AI Software Architect | RAG, MCP, eval, Go, Python |
| `energy` | Senior Quantitative Risk & Energy Systems Architect | Founder & Principal Risk Architect | PPA, weather, risk, hedging |
| `consulting` | Programmer, Engineer, Consultant | Founder | balanced, every bullet |

Lens keywords are matched word-bounded, so “RAG” does not match “storage”. An
employer whose bullets never mention the keywords keeps all of them: shrinking
someone’s history silently is worse than a longer CV.

### Auto-fit instead of manual nudging

Every job and education entry is a **locked two-column grid**: the company and
title are one text box at x=52, the dates one right-aligned box whose right edge
is the page margin, both on the same baseline. A long German compound therefore
wraps inside the left column and can never push a date inward.

The compiler solves the page budget *before* pdfcpu runs, so tightening costs
nothing: it lays out the whole CV in memory, and if the result exceeds the
budget it walks a density ladder until it fits.

| Preset | Body | Leading | Section gap |
| :--- | :--- | :--- | :--- |
| `10.0pt` | 10.0 | 12.4 | 13.0 |
| `9.5pt` | 9.5 | 11.7 | 10.0 |
| `9.0pt` | 9.0 | 11.0 | 8.0 |

`--density 9.5` forces one preset; without it the solver stops at the first fit
and reports `cv CV_Matt_Ji.pdf · fitted to 2 page(s) · 9.5pt`. The deckbuilder in
the TUI shows `fits on 2 page(s) at 10.0pt` after every change.

### The layout is a file you can read

Every build writes the exact `pdfcpu create` document next to the PDF:

```
applications/2026-10-24-testgmbh/layout_cv.json
```

`task layout` prints it, `pdfcpu create layout_cv.json out.pdf` reproduces the
page, and `--guides` (or the guides entry in the TUI menu) sets `"guides": true` plus
`"contentBox": true` so the grid and the text rail are drawn *into* the PDF and
visible in mupdf alongside the content.

### Commands

```bash
workbench                                   # TUI: new/switch capsule, tailor, letter, export
task app                                    # same, through task
workbench build --lens=ai --lang=de --company=TestGmbH
workbench build --company=TestGmbH --guides # blueprint grid inside the PDF
workbench build --company=TestGmbH --density 9.5
workbench new --company=TestGmbH --lens=energy
task app:build COMPANY=TestGmbH LENS=energy LANG=de BUDGET=2 GUIDES=1
task layout LAYOUT=dist/pdfcpu_cv_layout_de.json
workbench list
workbench import-pdf CV_Matt_Ji.pdf
```

`build` creates today’s capsule for the company if none exists (scaffolding the
cover letter from the lens highlights), reuses it if one does, then compiles the
layout, renders with `pdfcpu create`, attaches the state, and merges the package.
The same artifacts are mirrored to `dist/` under the pipeline names
(`CV_Matt_Ji_de.pdf`, `pdfcpu_cv_layout_de.json`, `Bewerbungsmappe_Matt_Ji_de.pdf`);
`dist/` is disposable, the capsule stays authoritative.

## The state files

| File | Owner | What it holds |
| :--- | :--- | :--- |
| `resume.json` | you | Source of truth. Never rewritten by a build. |
| `i18n_cache.json` | `i18n-sync`, you | Flat array of translations, keyed by `sha256(en)[:12]`. |
| `loadouts.json` | `loadout-edit` | Named decks: persona + selected highlight indices. |
| `glossary.json` | you | Terms `trans` must not mangle (acronyms, product names). |
| `lenses.json` | you | Narrative lenses: persona, title overrides, keyword priorities. |
| `applications/<date>-<company>/` | `workbench` | One application capsule (see above). |

Nothing in `dist/` is authoritative. `dist/resume-<lang>.json` is the exact
document a PDF was compiled from, and `--attach-json` embeds it in the PDF, so a
CV you sent two months ago can be unpacked back into data.

## Translation: content-addressed, not file-based

Translating `resume.json` into `resume.de.json` guarantees drift: edit one
English bullet and the German file is silently stale. Instead each English
string is hashed and looked up:

- **cache hit, `verified: true`** — your `de_override` is used. No network.
- **cache hit, unverified** — the cached `de_auto` is used. No network.
- **cache miss** — a new or edited English string. Only that string goes
  through `trans -b -no-ansi en:de`, and the result is stored unverified.

Old hashes are marked `"orphaned": true`, never deleted, so undoing an edit is a
cache hit rather than a retranslation.

```bash
task i18n:sync           # fill in what is missing (250 ms between calls)
task i18n:sync:dry       # see what would happen; --mock needs no network
task i18n:review         # huh deck: edit, mark verified, saved per item
task stats               # coverage + orphans
```

Prefer a spreadsheet? `i18n_cache.json` is a flat array on purpose:

```bash
task i18n:vd                      # vd i18n_cache.json, edit de_override in place
task i18n:fzf                     # fuzzy-pick one entry, open in $EDITOR
jq -r '.[] | select(.verified|not) | [.path_hint, .en] | @tsv' i18n_cache.json
```

## Loadouts: a deck, not an edit

A loadout names a persona (`basics.label` or one of `basics.altLabels`) and the
highlight indices worth showing for a given kind of job. Applying one trims the
export; `resume.json` keeps every bullet.

```bash
task loadout PROFILE=ai-systems    # multi-select per employer and project
task render LANG=de PROFILE=ai-systems -- --out dist/view.json
task build:de PROFILE=ai-systems
```

A work or project key missing from a loadout keeps all its bullets, so a new
role never disappears from your CV because you forgot to re-edit the deck.

## Commands

```
resumectl i18n-sync [--dry-run] [--mock] [--delay 250ms]
resumectl i18n-review [--path work]
resumectl stats
resumectl loadout-edit [--profile NAME]
resumectl render-json --lang en|de [--profile NAME] [--out -|PATH]
resumectl build-pdf --lang en|de [--stamp-sig] [--attach-json] [--reload-mupdf]
resumectl mappe
resumectl unpack FILE.pdf [--dir DIR]
resumectl tui
```

`build-pdf` is the full chain:

1. export the loadout + language view to `dist/resume-<lang>.json`
2. `typst compile --root . --input json_path=…`
3. `pdfcpu stamp add` the signature (last page, bottom right) when `--stamp-sig`
4. `pdfcpu attachments add` the exported JSON when `--attach-json`
5. `pdfcpu properties add` language, profile, synced task, build timestamp
6. `pdfcpu optimize`
7. `pdfcpu info` for the page count, warning above the page budget
8. `pkill -HUP mupdf` so an open viewer reloads without losing its viewport

`mappe` builds the German CV and merges `assets/anschreiben.pdf` and
`assets/zeugnisse.pdf` around it when they exist.

## Layout

```
cmd/resumectl/         CLI entrypoint (flags for everything)
internal/resume/       lossless JSON tree + translatable-path walker
internal/i18n/         content-addressed cache, translators, glossary
internal/loadout/      named decks, non-destructive apply
internal/export/       loadout + language -> JSON bytes
internal/pdf/          typst + pdfcpu + mupdf pipeline
internal/tui/          huh review deck and loadout editor
templates/resume.typ   bilingual A4 template, reads json_path
```

The resume is decoded into `map[string]any`, not a struct, so unknown keys
(`basics.signature`, `meta.rezi_source_id`, whatever a future tool adds) survive
every round trip. `go test ./...` includes a lossless round-trip test for
exactly that.

## Requirements

- Nix (recommended): `nix develop` provides everything at pinned versions.
- Otherwise: Go 1.23+, `typst`, `pdfcpu`, `mupdf`, `translate-shell`, `task`,
  and `visidata`/`fzf` only if you want the spreadsheet and picker workflows.

## Tests

```bash
task test    # go test ./...
task vet
```

`internal/pdf` is tested against stub `typst`/`pdfcpu` binaries on `PATH`, so
argument wiring and failure handling are covered without a PDF toolchain.

## Legacy

The older LaTeX/pandoc letter pipeline is kept in `Taskfile.legacy.yml` and
`src/` for the archive; the encrypted assets under `src/assets/` still need
their sops/age keys.
