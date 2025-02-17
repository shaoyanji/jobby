{
  description = "LaTeX and CV Management Platform";
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    #nixpkgs.url = github:NixOS/nixpkgs/nixos-21.05;
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
  }: let
    supportedSystems = [
      "x86_64-linux"
      # "x86_64-darwin"
      "aarch64-linux"
      # "aarch64-darwin"
    ];
    forAllSystems = nixpkgs.lib.genAttrs supportedSystems;

    nixpkgsFor = forAllSystems (system:
      import nixpkgs {
        inherit system;
        overlays = [
          (final: prev: {
            #          go = prev.go_1_22;
          })
        ];
      });
  in
    with flake-utils.lib;
      eachSystem allSystems (
        system: let
          pkgs = nixpkgs.legacyPackages.${system};
          # lib = pkgs.lib;

          tex = pkgs.texlive.combine {
            inherit
              (pkgs.texlive)
              scheme-minimal
              latex-bin
              latexmk
              biblatex
              biblatex-ieee
              enumitem
              fontaxes
              hyperref
              opensans
              titlesec
              xkeyval
              xcolor
              fontawesome5
              amsmath
              bookmark
              ;
          };
          #      packagingLib = import ./lib/packaging.nix {
          #        inherit pkgs tex;
          #        directory = ./.;
          #      };
          #
          #      documentationLib = import ./lib/documentation.nix {inherit self pkgs lib;};
          #      examplesLib = import ./lib/examples.nix {inherit self system pkgs lib;};
          #
          #      moduleMarkdownDocs = documentationLib.generateMarkdownDocumentation {};
          #      examplePdfDocs = examplesLib.generateExamplesDocumentation {};
          #
          #      site-env = mkPoetryEnv {
          #        projectDir = self + /site;
          #        python = pkgs.python311;
          #      };
          #
          #      site = pkgs.stdenvNoCC.mkDerivation {
          #        name = "nixcv-site";
          #        src = self + "/site";
          #        nativeBuildInputs = [site-env];
          #
          #        buildPhase = ''
          #          cp -r ${moduleMarkdownDocs}/* ./docs/
          #          cp -r ${examplePdfDocs}/* ./docs/
          #          mkdir -p .cache/plugin/social
          #          cp ${pkgs.roboto}/share/fonts/truetype/Roboto-* .cache/plugin/social/
          #          mkdocs build --site-dir dist
          #        '';
          #        installPhase = ''
          #          mkdir $out
          #          cp -R dist/* $out/
          #        '';
          #      };
        in rec {
          devShells = {
            default =
              import ./shell.nix {inherit pkgs;};
            texlive = pkgs.mkShell {
              packages = [
                pkgs.pandoc
                tex
              ];
            };
            cmark = pkgs.mkShell {
              packages = [
                pkgs.cmark
              ];
            };
          };

          packages = {
            document = pkgs.stdenvNoCC.mkDerivation rec {
              name = "cmark";
              src = self;
              buildInputs = [
                pkgs.coreutils
                pkgs.cmark
                pkgs.pandoc
                pkgs.wkhtmltopdf
                tex
                #                pkgs.texlive.combined.scheme-small
              ];
              phases = ["unpackPhase" "buildPhase" "installPhase"];
              buildPhase =
                /*
                bash
                */
                ''
                  export PATH="${pkgs.lib.makeBinPath buildInputs}";
                  mkdir -p .cache/texmf-var
                  env TEXMFHOME=.cache TEXMFVAR=.cache/texmf-var \
                  cmark -t html ./src/resume.md | pandoc --pdf-engine wkhtmltopdf -o document.pdf
                '';
              installPhase = ''
                mkdir -p $out
                cp document.pdf $out/
              '';
            };
          };
          defaultPackage = packages.document;

          # Tutorial for minimal LaTeX document
          #  packages =
          #    {
          #      document = pkgs.stdenvNoCC.mkDerivation rec {
          #        name = "latex-demo-document";
          #        src = self;
          #        buildInputs = [pkgs.coreutils tex];
          #        phases = ["unpackPhase" "buildPhase" "installPhase"];
          #        buildPhase = /* bash */ ''
          #          export PATH="${pkgs.lib.makeBinPath buildInputs}";
          #          mkdir -p .cache/texmf-var
          #          env TEXMFHOME=.cache TEXMFVAR=.cache/texmf-var \
          #          latexmk -interaction=nonstopmode -pdf -lualatex document.tex -f
          #        '';
          #        installPhase = ''
          #          mkdir -p $out
          #          cp document.pdf $out/
          #          #cp -r $src/* $out
          #          #                    chmod +x $out/share/doc/latex-demo/document.sh
          #        '';
          #      };
          #    };
          #  defaultPackage = packages.document;

          #      packages =
          #        {
          #          document = pkgs.stdenvNoCC.mkDerivation rec {
          #            name = "latex-demo-document";
          #            src = self;
          #            propagatedBuildInputs = [pkgs.coreutils pkgs.biber tex];
          #            phases = ["unpackPhase" "buildPhase" "installPhase"];
          #            SCRIPT = ''
          #              #!/bin/bash
          #              prefix=${builtins.placeholder "out"}
          #              export PATH="${pkgs.lib.makeBinPath propagatedBuildInputs}";
          #              DIR=$(mktemp -d)
          #              RES=$(pwd)/document.pdf
          #              cd $prefix/share
          #              mkdir -p "$DIR/.texcache/texmf-var"
          #              env TEXMFHOME="$DIR/.cache" \
          #                  TEXMFVAR="$DIR/.cache/texmf-var" \
          #                latexmk -interaction=nonstopmode -pdf -pdflatex \
          #                -output-directory="$DIR" \
          #                document.tex
          #              mv "$DIR/document.pdf" $RES
          #              rm -rf "$DIR"
          #            '';
          #            buildPhase = ''
          #              printenv SCRIPT >latex-demo-document
          #            '';
          #            installPhase = ''
          #              mkdir -p $out/{bin,share}
          #              cp ./old/document.tex $out/share/document.tex
          #              cp ./old/cv.tex $out/share/cv.tex
          #              cp ./old/self.bib $out/share/self.bib
          #              cp latex-demo-document $out/bin/latex-demo-document
          #              chmod u+x $out/bin/latex-demo-document
          #            '';
          #          };
          #        }
          #        // packagingLib.examples
          #        // {
          #          inherit
          #            moduleMarkdownDocs
          #            examplePdfDocs
          #            site
          #            ;
          #        };
          #
          #      apps = {
          #        copyMarkdownDocs = {
          #          type = "app";
          #          program = "${documentationLib.copyMarkdownDocs}/bin/copy-markdown-docs";
          #        };
          #        copyExampleDocs = {
          #          type = "app";
          #          program = "${examplesLib.copyExampleDocs}/bin/copy-example-docs";
          #        };
          #      };
          #
          #      extra = {
          #        evaluatedModules = documentationLib.evaluatedModules {};
          #      };
          #
          #      checks =
          #        {}
          #        // packagingLib.examples
          #        // {
          #          inherit
          #            moduleMarkdownDocs
          #            examplePdfDocs
          #            ;
          #        };
          #
          #      test = lib.listToAttrs (
          #        builtins.map
          #        (
          #          file:
          #            lib.nameValuePair
          #            (lib.removeSuffix ".nix" file)
          #            (import ./test/${file} {inherit lib;})
          #        )
          #        (
          #          builtins.attrNames (
          #            lib.filterAttrs
          #            (name: value: value == "regular")
          #            (builtins.readDir ./test)
          #          )
          #        )
          #      );
          #    })
          #    // {
          #      nixosModules.templates.simple = ./templates/simple.nix;
          #    };
        }
      );
}
