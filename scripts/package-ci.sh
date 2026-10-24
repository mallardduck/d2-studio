#!/usr/bin/env bash
# Gathers one platform's build outputs into dist/ for upload: the archives and
# installers, each with the license and third-party notices, and a checksum file.
#
#   scripts/package-ci.sh <macos|windows|linux> <version-label>
#
# Run it after the platform's package tasks (see docs/building.md). The CI
# workflow uses it, and it can be run locally to check what a release would
# contain. Required outputs missing => it fails; optional ones (installers,
# AppImage) are included only if the build produced them.
set -euo pipefail

platform=${1:?usage: package-ci.sh <macos|windows|linux> <version-label>}
version=${2:?usage: package-ci.sh <macos|windows|linux> <version-label>}
arch=${ARCH:-amd64}

cd "$(dirname "$0")/.."
root=$PWD
dist=$root/dist
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
rm -rf "$dist"
mkdir -p "$dist"

die() { echo "package-ci: $*" >&2; exit 1; }
need() { [ -e "$1" ] || die "missing $1; run the platform's package task first"; }

# The notices that must travel with every distribution.
go run ./tools/gennotices -o "$work/THIRD_PARTY_NOTICES.md" -full "$work/THIRD_PARTY_LICENSES.txt" >/dev/null
add_docs() {
  local dir=$1
  cp LICENSE "$dir/"
  cp "$work/THIRD_PARTY_NOTICES.md" "$work/THIRD_PARTY_LICENSES.txt" "$dir/"
  mkdir -p "$dir/third_party/d2"
  cp third_party/d2/THIRD_PARTY_NOTICES.txt "$dir/third_party/d2/"
}

make_zip() { # make_zip <dir-to-archive> <absolute-output.zip>
  local src=$1 out=$2
  if command -v zip >/dev/null 2>&1; then
    (cd "$(dirname "$src")" && zip -qry "$out" "$(basename "$src")")
  elif command -v 7z >/dev/null 2>&1; then
    (cd "$(dirname "$src")" && 7z a -tzip -bso0 -bsp0 "$out" "$(basename "$src")")
  else
    powershell -NoProfile -Command "Compress-Archive -Path '$src' -DestinationPath '$out'"
  fi
}

case $platform in
  macos)
    need bin/d2-studio.app
    name=d2-studio-$version-macos-universal
    stage=$work/$name
    mkdir -p "$stage"
    # ditto keeps the bundle's symlinks, permissions and signature intact.
    ditto bin/d2-studio.app "$stage/D2 Studio.app"
    add_docs "$stage"
    ditto -c -k --keepParent "$stage" "$dist/$name.zip"
    ;;
  windows)
    need bin/d2-studio.exe
    name=d2-studio-$version-windows-$arch
    stage=$work/$name
    mkdir -p "$stage"
    cp bin/d2-studio.exe "$stage/"
    add_docs "$stage"
    make_zip "$stage" "$dist/$name-portable.zip"
    for f in bin/*-installer.exe; do
      [ -e "$f" ] && cp "$f" "$dist/$name-setup.exe"
    done
    ;;
  linux)
    need bin/d2-studio
    name=d2-studio-$version-linux-$arch
    stage=$work/$name
    mkdir -p "$stage"
    cp bin/d2-studio "$stage/"
    add_docs "$stage"
    tar -C "$work" -czf "$dist/$name.tar.gz" "$name"
    for f in bin/*.deb; do
      [ -e "$f" ] && cp "$f" "$dist/$name.deb"
    done
    for f in bin/*.AppImage; do
      [ -e "$f" ] && cp "$f" "$dist/$name.AppImage"
    done
    ;;
  *) die "unknown platform '$platform' (macos, windows or linux)" ;;
esac

# Checksums of everything in dist/ (not of the checksum file itself).
sums=SHA256SUMS-$platform.txt
(
  cd "$dist"
  if command -v sha256sum >/dev/null 2>&1; then sha256sum -- * ; else shasum -a 256 -- * ; fi >"$work/$sums"
)
mv "$work/$sums" "$dist/$sums"

echo "package-ci: $platform $version"
(cd "$dist" && ls -l | tail -n +2 | awk '{print "  " $5 "\t" $9}')
