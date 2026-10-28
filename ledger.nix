{ pkgs ? import <nixpkgs> { } }:

with pkgs;

let
  version = "3.4.1";
in
pkgsStatic.stdenv.mkDerivation {
  pname = "ledger";
  inherit version;

  src = fetchFromGitHub {
    owner = "ledger";
    repo = "ledger";
    tag = "v${version}";
    hash = "sha256-yk6/4ImUzgZY8O7MmQMwFkuJ/pMXo6W5TAA0GGIxYgg=";
  };

  outputs = [ "out" "dev" ];

  buildInputs = [ pkgsStatic.gmp pkgsStatic.mpfr gnused pkgsStatic.boost ];
  # nixpkgs 26.05 no longer propagates gmp through mpfr for static links.
  NIX_LDFLAGS = "-lgmp";

  nativeBuildInputs = [ cmake tzdata ];

  cmakeFlags = [
    "-DCMAKE_INSTALL_LIBDIR=lib"
    "-DBUILD_DOCS:BOOL=OFF"
    "-DUSE_PYTHON:BOOL=OFF"
    "-DUSE_GPGME:BOOL=OFF"
    "-DBUILD_LIBRARY:BOOL=OFF"
  ];

  enableParallelBuilding = true;

  installTargets = [ "install" ];

  checkPhase = ''
    runHook preCheck
    env LD_LIBRARY_PATH=$PWD \
      DYLD_LIBRARY_PATH=$PWD \
      ctest -j$NIX_BUILD_CORES
    runHook postCheck
  '';

  doCheck = true;

}
