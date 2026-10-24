# CRUSH.md — Jobby Development Guide

## Build commands
- `nix develop` — dev shell: go, pdfcpu 0.16 (create engine), mupdf, translate-shell, task
- `task` — list every pipeline task
- `task build:en` / `task build:de` — export → typst → pdfcpu → mupdf reload
- `task app` — application workbench TUI
- `task app:guides COMPANY=X` — rebuild with pdfcpu alignment guides
- `task layout LAYOUT=dist/pdfcpu_cv_layout_de.json` — inspect the create JSON
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
- `internal/layout/` — flow-to-coordinate compiler for `pdfcpu create` (metrics, wrap, paging, density solver)
- `internal/pdf/engine.go` — pdfcpu engine: create, validate, attach, properties, merge
- `internal/tui/` — huh review deck and loadout editor
- `internal/capsule/` — application folders, `capsule.json`, import/export
- `internal/lens/` — narrative lenses: persona, titles, keyword priorities
- `internal/coverletter/` — Anschreiben model, scaffold, translatable refs
- `internal/workbench/` — draft, build (CV + letter + package), TUI
- `internal/layout/compiler.go` — y-cursor layout, keep-together, page budget
- `templates/resume.typ`, `templates/style.typ` — legacy resumectl typst template (workbench does not use typst)
- `glossary.json` — terms `trans` must not mangle
- `flake.nix` — the only dependency source
