#!/usr/bin/env bash
# Installs the system packages the Linux build and its checks need (GTK 4,
# WebKitGTK 6.0, a virtual display for the smoke test).
#
# GitHub's Ubuntu mirror sometimes slows to a crawl and can stall for minutes
# on a single file, so every download has a short timeout and is retried, and
# the downloaded packages live in $APT_ARCHIVES, which CI caches between runs.
set -euo pipefail

archives="${APT_ARCHIVES:-$HOME/apt-archives}"
mkdir -p "$archives/partial"

opts=(
  -o "Dir::Cache::archives=$archives"
  -o Acquire::Retries=5
  -o Acquire::http::Timeout=20
  -o Acquire::https::Timeout=20
  -o APT::Sandbox::User=root
  -o Dpkg::Use-Pty=0
)

sudo apt-get "${opts[@]}" update
sudo apt-get "${opts[@]}" install -y --no-install-recommends \
  build-essential pkg-config libgtk-4-dev libwebkitgtk-6.0-dev xvfb libfuse2t64

# apt ran as root; hand the files back so the cache can read and save them.
sudo chown -R "$(id -u):$(id -g)" "$archives"
rm -rf "$archives/partial" "$archives/lock"
