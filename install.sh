#!/usr/bin/env sh
set -eu

REPO_URL="${F_REPO_URL:-https://github.com/sergeybychkovvvpgroup-beep/f.git}"
RAW_BASE="${F_RAW_BASE:-https://raw.githubusercontent.com/sergeybychkovvvpgroup-beep/f/main}"
BIN_DIR="${F_BIN_DIR:-$HOME/.local/bin}"
CACHE_DIR="${F_CACHE_DIR:-$HOME/.cache/f/source}"
BIN="$BIN_DIR/f"

mkdir -p "$BIN_DIR"

need() {
  command -v "$1" >/dev/null 2>&1
}

if need curl; then
  tmp="$(mktemp)"
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) asset="f-linux-amd64" ;;
    aarch64|arm64) asset="f-linux-arm64" ;;
    *) asset="" ;;
  esac
  if [ -n "$asset" ] && curl -fsSL "$RAW_BASE/dist/$asset" -o "$tmp" 2>/dev/null; then
    install -m 0755 "$tmp" "$BIN"

    rm -f "$tmp"
    echo "installed: $BIN"
    echo "next: f setup <hosts-repo-url>"
    exit 0
  fi
  rm -f "$tmp"
fi

if ! need git; then
  echo "error: git is required for source install" >&2
  exit 1
fi
if ! need go; then
  echo "error: Go is required for source install (or publish dist/f-linux-* for binary install)" >&2
  exit 1
fi

if [ -d "$CACHE_DIR/.git" ]; then
  git -C "$CACHE_DIR" remote set-url origin "$REPO_URL"
  git -C "$CACHE_DIR" pull --ff-only
else
  rm -rf "$CACHE_DIR"
  mkdir -p "$(dirname "$CACHE_DIR")"
  git clone --depth 1 "$REPO_URL" "$CACHE_DIR"
fi

(cd "$CACHE_DIR" && commit="$(git rev-parse HEAD)" && go build -buildvcs=false -ldflags "-X f/internal/app.buildCommit=$commit" -o "$BIN" ./cmd/f)

echo "installed: $BIN"
echo "ensure PATH contains: $BIN_DIR"
echo "next: f setup <hosts-repo-url>"
