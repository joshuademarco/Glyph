#!/usr/bin/env bash

set -euo pipefail

VERSION="${1:-$(git describe --tags --always 2>/dev/null || echo dev)}"
OUT="dist"
mkdir -p "$OUT"

targets=(
  linux/amd64 linux/arm64 linux/arm linux/386 linux/riscv64
  darwin/amd64 darwin/arm64
  windows/amd64 windows/arm64 windows/386
  freebsd/amd64 freebsd/arm64
  openbsd/amd64 netbsd/amd64
)

for t in "${targets[@]}"; do
  os="${t%/*}"; arch="${t#*/}"
  ext=""; [ "$os" = "windows" ] && ext=".exe"
  name="$OUT/glyph_${VERSION}_${os}_${arch}${ext}"
  echo "building $name"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "$name" .
done

echo "done — binaries in ./$OUT"
