# CRUSH.md — Jobby Development Guide

## Build commands
- `nix develop` — dev shell: go, typst, pdfcpu, mupdf, translate-shell, visidata, task
- `task` — list every pipeline task
- `task build:en` / `task build:de` — export → typst → pdfcpu → mupdf reload
- `task app` — application workbench TUI
- `task app:build COMPANY=X LENS=ai LANG=de` — capsule → CV + letter + package
- `task app:list` — capsules under `applications/`
- `task app:import PDF=file.pdf` — restore a capsule from an old PDF
- `task mappe` — German CV merged with Anschreiben and Zeugnisse
- `nix build .#resumectl .#workbench` — package the CLIs with their runtime deps

## Test / lint commands
- `task test` — `go test ./...` (pdf tests use stub typst/pdfcpu binaries)
- `task vet`
- `task fmt` — `gofmt -w cmd internal`
- `pdfcpu validate dist/resume-en.pdf` — validate a built PDF

## Code style
- **Go**: standard `gofmt`, table-free tests with `t.TempDir()` fixtures
- **JSON**: two-space indent, trailing newline, unknown keys preserved
- **YAML**: 2-space indentation, lowercase keys
- **Naming**: kebab-case for files, snake_case for JSON cache fields
- **Secrets**: always sops; `resume.json` is not encrypted

## Key files
- `cmd/resumectl/` — CLI: flags first, huh forms optional
- `internal/resume/` — lossless JSON tree + translatable-path walker
- `internal/i18n/` — content-addressed cache (`i18n_cache.json`)
- `internal/loadout/` — named decks (`loadouts.json`)
- `internal/export/` — loadout + language → JSON view
- `internal/pdf/` — typst + pdfcpu + mupdf pipeline
- `internal/tui/` — huh review deck and loadout editor
- `internal/capsule/` — application folders, `capsule.json`, import/export
- `internal/lens/` — narrative lenses: persona, titles, keyword priorities
- `internal/coverletter/` — Anschreiben model, scaffold, translatable refs
- `internal/workbench/` — draft, build (CV + letter + package), TUI
- `internal/pdf/autofit.go` — page-budget solver over the density ladder
- `templates/style.typ` — shared palette, header, section and date helpers
- `templates/resume.typ` — bilingual A4 CV, reads `json_path`
- `templates/cover_letter.typ` — Anschreiben on the same header
- `glossary.json` — terms `trans` must not mangle
- `flake.nix` — the only dependency source
