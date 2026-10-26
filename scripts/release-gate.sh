#!/usr/bin/env bash
# Decides whether a release may be published automatically or must stay a draft
# for a person to review.
#
#   scripts/release-gate.sh <dist-dir> <gates-dir> <tag> <auto-publish-flag>
#
# Prints two lines for $GITHUB_OUTPUT:  publish=true|false  and  reason=<why>.
# It always exits 0 when it can decide; "do not publish" is an answer, not an
# error. The safe answer is the default: a draft.
#
# Publishing automatically needs ALL of:
#   - a pre-release tag (it has a suffix, e.g. v0.1.0-rc.1); stable tags are
#     always drafts
#   - the repository switch AUTO_PUBLISH_PRERELEASES set to exactly "true"
#   - every file a release must contain
#   - a smoke test that passed on every platform
set -euo pipefail

dist=${1:?usage: release-gate.sh <dist-dir> <gates-dir> <tag> <auto-publish-flag>}
gates=${2:?usage: release-gate.sh <dist-dir> <gates-dir> <tag> <auto-publish-flag>}
tag=${3:?usage: release-gate.sh <dist-dir> <gates-dir> <tag> <auto-publish-flag>}
auto=${4:-}

decide() { echo "publish=$1"; echo "reason=$2"; exit 0; }

case "$tag" in
  *-*) ;; # has a suffix: a pre-release
  *) decide false "stable releases are always drafts for a person to publish" ;;
esac

[ "$auto" = "true" ] || decide false "the AUTO_PUBLISH_PRERELEASES repository variable is not set to true"

# Files every release must have (the AppImage is optional).
missing=()
for pattern in \
  "*-macos-universal.zip" \
  "*-windows-amd64-setup.exe" "*-windows-amd64-portable.zip" \
  "*-linux-amd64.tar.gz" "*-linux-amd64.deb" \
  "SHA256SUMS.txt"; do
  # shellcheck disable=SC2086
  if ! ls "$dist"/$pattern >/dev/null 2>&1; then missing+=("$pattern"); fi
done
if [ ${#missing[@]} -gt 0 ]; then
  decide false "missing release files: ${missing[*]}"
fi

for platform in macos windows linux; do
  result=$(cat "$gates/SMOKE-$platform.txt" 2>/dev/null || echo "not recorded")
  [ "$result" = "success" ] || decide false "the $platform smoke test did not pass ($result)"
done

decide true "pre-release, switch on, all files present, smoke tests passed on every platform"
