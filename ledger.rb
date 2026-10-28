class Ledger < Formula
  desc "Command-line, double-entry accounting tool"
  homepage "https://ledger-cli.org/"
  license "BSD-3-Clause"
  head "https://github.com/ledger/ledger.git", branch: "master"

  stable do
    url "https://github.com/ledger/ledger/archive/refs/tags/v3.4.1.tar.gz"
    sha256 "1cf012cdc8445cab0efc445064ef9b2d3f46ed0165dae803c40fe3d2b23fdaad"
  end

  livecheck do
    url :stable
    regex(/^v?(\d+(?:\.\d+)+)$/i)
  end

  depends_on "boost"
  depends_on "gmp"
  depends_on "mpfr"
  depends_on "python@3.14" => :build
  depends_on "cmake" => :build

  def install
    ENV.cxx11
    ENV.prepend_path "PATH", Formula["python@3.14"].opt_libexec/"bin"

    # Use architecture-specific Homebrew archives instead of a hard-coded path.
    # This keeps GMP and MPFR out of the shipped binary's dynamic dependencies.
    mpfr_lib = formula_opt_lib("mpfr")/"libmpfr.a"
    gmp_lib = formula_opt_lib("gmp")/"libgmp.a"
    inreplace "CMakeLists.txt" do |s|
      s.gsub!(
        "target_link_libraries(${_target} ${MPFR_LIB})",
        "target_link_libraries(${_target} #{mpfr_lib})",
      )
      s.gsub!(
        "target_link_libraries(${_target} ${GMP_LIB})",
        "target_link_libraries(${_target} #{gmp_lib})",
      )
      # Ledger's optional edit support links Apple's dynamic libedit.tbd.
      s.gsub!(
        'check_library_exists(edit add_history "" HAVE_EDIT)',
        "set(HAVE_EDIT 0)",
      )
    end
    args = %W[
      --jobs=#{ENV.make_jobs}
      --output=build
      --prefix=#{prefix}
      --boost=#{Formula["boost"].opt_prefix}
      --
      -DCMAKE_FIND_LIBRARY_SUFFIXES=.a
      -DCMAKE_POLICY_VERSION_MINIMUM=3.5
      -DBUILD_SHARED_LIBS:BOOL=OFF
      -DBUILD_DOCS:BOOL=OFF
      -DBoost_NO_BOOST_CMAKE=ON
      -DUSE_PYTHON:BOOL=OFF
      -DUSE_GPGME:BOOL=OFF
      -DBUILD_LIBRARY:BOOL=OFF
      -DBoost_USE_STATIC_LIBS:BOOL=ON
    ] + std_cmake_args

    system "./acprep", "opt", "make", *args
    system "./acprep", "opt", "make", "install", *args
  end

  test do
    balance = testpath/"output"
    system bin/"ledger",
      "--args-only",
      "--file", pkgshare/"examples/sample.dat",
      "--output", balance,
      "balance", "--collapse", "equity"
    assert_equal "          $-2,500.00  Equity", balance.read.chomp
  end
end
