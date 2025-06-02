{
  description = "Jobby";
  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
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
              koma-script
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
            min = pkgs.mkShellNoCC {
              packages = [
                pkgs.pandoc
                pkgs.wkhtmltopdf
                pkgs.ghostscript
                pkgs.pdfcpu
                pkgs.sops
                pkgs.age
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
            cv = pkgs.stdenvNoCC.mkDerivation rec {
              name = "cv";
              src = ./src;

              buildInputs = [
                pkgs.coreutils
                pkgs.pandoc
                pkgs.wkhtmltopdf
                pkgs.ghostscript
                pkgs.pdfcpu
                pkgs.sops
                pkgs.age
                tex
              ];
              encryptflags = "-upw decrypt -opw hdjsfhiuvhu843hui123251";
              gen = ''
                pandoc -f markdown \
                  -t pdf \
                  --pdf-engine wkhtmltopdf \
                  -o document.pdf \
                  -c ./lib/resume-stylesheet.css -s \
                  begin.md
              '';
              phases = ["unpackPhase" "patchPhase" "buildPhase" "installPhase"];
              patchPhase = ''
                export XDG_CONFIG_HOME=./.config
                age --decrypt -i .config/sops/age/keys.txt assets/enc.tar.gz.age > unenc.tar.gz
                tar -xzf unenc.tar.gz
                touch begin.md
                echo "---" >> begin.md
                sops -d -a .config/sops/age/keys.txt ./assets/resume.enc.yaml >> begin.md
                echo "..." >> begin.md
                cat ./unenc/resume.md >> begin.md
              '';
              buildPhase = ''
                 export PATH="${pkgs.lib.makeBinPath buildInputs}:bin";
                 mkdir -p .cache/texmf-var .cache/pdfcpu
                 env TEXMFHOME=.cache TEXMFVAR=.cache/texmf-var \
                 ${gen}
                shrinkpdf.sh document.pdf > shrink.pdf
                 pdfcpu encrypt -c .cache/pdfcpu ${encryptflags} shrink.pdf final.enc.pdf
                 pdfcpu optimize -c .cache/pdfcpu shrink.pdf final.pdf
              '';
              installPhase = ''
                mkdir -p $out
                cp {final,final.enc}.pdf $out/
              '';
            };

            letter = pkgs.stdenvNoCC.mkDerivation rec {
              name = "letter";
              src = ./src;

              buildInputs = [
                pkgs.coreutils
                pkgs.pandoc
                pkgs.ghostscript
                pkgs.pdfcpu
                pkgs.sops
                pkgs.age
                tex
              ];
              encryptflags = "-upw decrypt -opw hdjsfhiuvhu843hui123251";
              gen = ''
                pandoc -f markdown \
                -t pdf \
                --template ./lib/letter.latex \
                -o document.pdf \
                -s \
                begin.md
              '';
              phases = ["unpackPhase" "patchPhase" "buildPhase" "installPhase"];
              patchPhase = ''
                export XDG_CONFIG_HOME=./.config
                age --decrypt -i .config/sops/age/keys.txt assets/enc.tar.gz.age > unenc.tar.gz
                tar -xzf unenc.tar.gz
                touch begin.md
                echo "---" >> begin.md
                sops -d ./assets/letter.enc.yaml >> begin.md
                echo "..." >> begin.md
                cat ./unenc/letter.md >> begin.md
              '';
              buildPhase = ''
                export PATH="${pkgs.lib.makeBinPath buildInputs}:bin";
                mkdir -p .cache/texmf-var .cache/pdfcpu
                env TEXMFHOME=.cache TEXMFVAR=.cache/texmf-var \
                ${gen}
                                shrinkpdf.sh document.pdf > shrink.pdf
                pdfcpu encrypt -c .cache/pdfcpu ${encryptflags} shrink.pdf final.enc.pdf
                pdfcpu optimize -c .cache/pdfcpu shrink.pdf final.pdf
              '';
              installPhase = ''
                mkdir -p $out
                cp {final,final.enc}.pdf $out/
              '';
            };
          };
          defaultPackage = packages.cv;
          cv = packages.cv;
          letter = packages.letter;
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
