{
  description = "jobby — bilingual JSON Resume pipeline (resumectl)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    # pdfcpu create (the layout compiler's renderer) needs >= 0.16; the pinned
    # nixpkgs still ships 0.11, so the renderer comes from its own pin.
    nixpkgs-pdfcpu.url = "github:NixOS/nixpkgs/5adc50831c259f87faeb09ddf930d43597197473";
  };

  outputs = {
    self,
    nixpkgs,
    nixpkgs-pdfcpu,
    flake-utils,
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = nixpkgs.legacyPackages.${system};
        inherit (pkgs) lib;
        pdfcpu = nixpkgs-pdfcpu.legacyPackages.${system}.pdfcpu;

        # Everything resumectl shells out to. Wrapped into the binary's PATH so
        # `nix run .#` works outside the dev shell.
        runtimeDeps = with pkgs; [
          typst # only resumectl's legacy typst templates need this
          pdfcpu
          mupdf
          translate-shell
        ];		mkCli = pname: subPackage: meta: pkgs.buildGoModule {
          inherit pname;
          version = "1.0.0";
          src = ./.;
          subPackages = [subPackage];
          vendorHash = "sha256-kuheizBlwHUpozXPmFnoYOJ4zzyUDu3U5k8j0N1bhGc=";
          nativeBuildInputs = [pkgs.makeWrapper];
          postInstall = ''
            wrapProgram $out/bin/${pname} \
              --prefix PATH : ${lib.makeBinPath runtimeDeps}
          '';
          meta = meta // {mainProgram = pname; license = lib.licenses.mit; homepage = "https://github.com/shaoyanji/jobby";};
        };

        resumectl = mkCli "resumectl" "cmd/resumectl" {
          description = "Bilingual JSON Resume pipeline: loadouts, i18n cache, typst to PDF";
        };

        workbench = mkCli "workbench" "cmd/workbench" {
          description = "Application capsules: lenses, cover letters, auto-fitted PDF packages";
        };
      in {		packages = {
          inherit resumectl workbench;
          default = workbench;
        };

        apps = {
          inherit resumectl workbench;
          default = flake-utils.lib.mkApp {drv = workbench;};
        };

        devShells.default = pkgs.mkShellNoCC {
          packages = with pkgs;
            [
              go
              gopls
              gotools
              go-task
              typst
              mupdf
              poppler_utils
              translate-shell
              visidata
              jq
              fzf
            ]
            ++ runtimeDeps;
          shellHook = ''
            # GitHub images export GOROOT for their own Go; leaving it set makes
            # the toolchain here invoke the image's compiler and fail on version.
            unset GOROOT
            export JOBBY_ROOT="''${JOBBY_ROOT:-$PWD}"
            echo "jobby: dev shell (go, pdfcpu $(pdfcpu version | head -1 | cut -d' ' -f2), typst for legacy resumectl)"
            echo "  task            # pipeline tasks"
            echo "  task app        # workbench TUI"
            echo "  task app:build COMPANY=X LENS=ai LANG=de"
            command -v trans >/dev/null || echo "  warning: trans not on PATH"
          '';
        };

        formatter = pkgs.alejandra;
      }
    );
}
