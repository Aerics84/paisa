{
  description = "paisa";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-26.05";
  # Pin hldeger to 1.32.x; 1.40 has a bug related to chained price calculation
  # https://github.com/simonmichael/hledger/issues/2254
  inputs.hledger-pkgs.url =
    "github:NixOS/nixpkgs/ebe4301cbd8f81c4f8d3244b3632338bbeb6d49c";

  outputs = { self, nixpkgs, flake-utils, hledger-pkgs }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        hledger = hledger-pkgs.legacyPackages.${system};
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
