{ pkgs ? import <nixpkgs> { }, hledger ? import <nixpkgs> { } }:

pkgs.mkShell {
  nativeBuildInputs = [
    # Go 1.26 matches go.mod; nixpkgs 26.05 provides go_1_26.
    pkgs.go_1_26
    pkgs.gotools
    pkgs.gopls
    pkgs.sqlite
    pkgs.nodejs_24
    pkgs.libuuid
    pkgs.bun
    # pkgs.pkgsCross.mingwW64.buildPackages.gcc

    pkgs.python3Packages.mkdocs-material
    pkgs.python3Packages.beancount
    pkgs.python3Packages.beanquery

    # test
    pkgs.ledger
    hledger.hledger
  ] ++ (pkgs.lib.optional pkgs.stdenv.isLinux pkgs.wails);

  shellHook = ''
    export CGO_ENABLED=1
  '';

  env = { LD_LIBRARY_PATH = pkgs.lib.makeLibraryPath [ pkgs.libuuid ]; };
}
