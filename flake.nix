{
  description = "paisa";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
  # hledger is gated on the chained-price regression; pin to the nixpkgs rev
  # that carries the candidate release.
  # https://github.com/simonmichael/hledger/issues/2254
  inputs.hledger-pkgs.url = "github:NixOS/nixpkgs/nixos-26.05";

  outputs = { self, nixpkgs, flake-utils, hledger-pkgs }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        hledgerPkgs = hledger-pkgs.legacyPackages.${system};
        # nixpkgs 26.05 (and master) still package hledger 1.52; pin the
        # 1.52.4 patch release from Hackage. Issue #2254 is fixed since 1.41.
        hledgerPackages = hledgerPkgs.haskellPackages.override {
          overrides = self: super: {
            hledger-lib = super.hledger-lib.overrideAttrs (old: {
              version = "1.52.4";
              src = pkgs.fetchurl {
                url = "mirror://hackage/hledger-lib-1.52.4.tar.gz";
                hash = "sha256-xwnEt/DBiC4i7EfB8x5cV0a/ISbLmY9gNqCZN/sb5s0=";
              };
            });
            hledger = super.hledger.overrideAttrs (old: {
              version = "1.52.4";
              src = pkgs.fetchurl {
                url = "mirror://hackage/hledger-1.52.4.tar.gz";
                hash = "sha256-e9HTlouijTPYF4Dk7QexKG87VP/+w7xNRHcLCTSukVg=";
              };
            });
          };
        };
        hledger = hledgerPackages;
        version = builtins.replaceStrings [ "\n" ] [ "" ] (builtins.readFile ./VERSION);
        paisaWeb = pkgs.buildNpmPackage {
          pname = "paisa-web";
          inherit version;
          src = ./.;
          nodejs = pkgs.nodejs_24;
          # sass-embedded ships a prebuilt Dart binary; without patching it is
          # bound to the FHS loader and dies with ENOENT on NixOS.
          nativeBuildInputs = [ pkgs.autoPatchelfHook ];
          buildInputs = [ pkgs.stdenv.cc.cc.lib ];
          preBuild = "autoPatchelf .";
          npmDepsHash = "sha256-NDIyTkTBHjUBYbukCp6HZoVyCRNNdwbz4nf4+URc30M=";
          npmBuildScript = "build";
          installPhase = ''
            runHook preInstall
            mkdir -p $out
            cp -r web/static $out/static
            runHook postInstall
          '';
        };
      in {
        devShells.default = import ./shell.nix {
          inherit pkgs;
          inherit hledger;
        };

        packages = {
          default = pkgs.buildGoModule {
            pname = "paisa-cli";
            meta.mainProgram = "paisa";
            inherit version;
            ldflags = [ "-X github.com/ananthakumaran/paisa/internal/version.current=${version}" ];

            src = ./.;

            vendorHash = "sha256-Cjqnoju7klZzFDVBa7m8RpQo6DqJruCO246zNGMAN1o=";

            env = {
              CGO_ENABLED = 1;
            };

            doCheck = false;

            subPackages = [ "." ];

            preConfigure = ''
              rm -rf web/static
              cp -r ${paisaWeb}/static web/static
            '';

          };

          "paisa-web" = paisaWeb;
        };
      });
}
