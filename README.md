# jobby

A composable framework for job applications: one JSON Resume source, two
languages, and a sub-second PDF loop you can drive from a TUI, a flag, or your
own editor.

`resumectl` is the whole engine. It is a plain Go CLI, so every action works
headless and scripts cleanly; the interactive `huh` forms are a convenience on
top, never the only way to change state.

## Why

A CV is data, but most tools make it a document. Once it is a document, every
application produces a new copy that drifts from the last one and you lose the
history of your own career arc.

jobby keeps the source in `resume.json` (JSON Resume schema plus a few custom
fields), keeps German translations in a content-addressed sidecar, and treats a
CV as a *view*: pick a loadout, pick a language, compile.

## Quick start

```bash
nix develop            # go, typst, pdfcpu, mupdf, translate-shell, visidata, task
task                   # list the pipeline
task build:en          # dist/resume-en.pdf
task build:de          # dist/resume-de.pdf
task preview           # mupdf + build, reloading in place
```

Without Nix, install `go`, `typst`, `pdfcpu`, `mupdf`, `translate-shell` and
`go-task` yourself, then `task` works the same.

There is no `nix` in a Codespace by default: `curl -L https://nixos.org/nix/install | sh -s -- --no-daemon`
followed by `. ~/.nix-profile/etc/profile.d/nix.sh` gives you the flake, and
`nix develop` then supplies every dependency at pinned versions.

## The four state files

| File | Owner | What it holds |
| :--- | :--- | :--- |
| `resume.json` | you | Source of truth. Never rewritten by a build. |
| `i18n_cache.json` | `i18n-sync`, you | Flat array of translations, keyed by `sha256(en)[:12]`. |
| `loadouts.json` | `loadout-edit` | Named decks: persona + selected highlight indices. |
| `glossary.json` | you | Terms `trans` must not mangle (acronyms, product names). |

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
