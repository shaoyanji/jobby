# CRUSH.md - Jobby Development Guide

## Build Commands
- `nix build .#cv` - Build CV PDF
- `nix build .#letter` - Build letter PDF
- `nix-shell` - Enter development shell
- `task generatecv` - Generate CV via task
- `task generateletter` - Generate letter via task

## Lint/Format Commands
- `pdfcpu validate result/final.pdf` - Validate PDF
- `pdfcpu optimize result/final.pdf` - Optimize PDF

## Test Commands
- Manual testing via mupdf result/final.pdf
- `task compress-assets` - Test asset compression

## Code Style
- **YAML**: 2-space indentation, lowercase keys
- **LaTeX**: Use provided templates in src/lib/
- **CSS**: Use resume-stylesheet.css conventions
- **Naming**: kebab-case for files, snake_case for vars
- **Secrets**: Always use sops for encryption

## Key Files
- `src/lib/letter.latex` - Letter template
- `src/lib/resume-stylesheet.css` - Resume styling
- `src/assets/*.enc.yaml` - Encrypted secrets
- `Taskfile.yml` - Build automation
- `flake.nix` - Nix build system