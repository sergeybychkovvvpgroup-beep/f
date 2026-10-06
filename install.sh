#!/usr/bin/env sh
set -eu

REPO="${F_GITHUB_REPO:-sergeybychkovvvpgroup-beep/f}"
API_URL="https://api.github.com/repos/$REPO/releases/latest"
BIN_DIR="${F_BIN_DIR:-$HOME/.local/bin}"

need() {
  command -v "$1" >/dev/null 2>&1
}

if ! need curl; then
  echo "error: curl is required" >&2
  exit 1
fi

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  aarch64|arm64) arch="arm64" ;;
  *)
    echo "error: unsupported architecture: $(uname -m)" >&2
    exit 1
    ;;
esac

release_json="$(curl -fsSL "$API_URL")"
tag="$(printf '%s\n' "$release_json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | sed -n '1p')"
if [ -z "$tag" ]; then
  echo "error: cannot determine latest release tag" >&2
  exit 1
fi
version="${tag#v}"
base="https://github.com/$REPO/releases/download/$tag"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT HUP INT TERM

install_deb() {
  deb="$tmpdir/f.deb"
  url="$base/f_${version}_linux_${arch}.deb"
  echo "downloading: $url"
  curl -fsSL "$url" -o "$deb"
  if [ "$(id -u)" -eq 0 ]; then
    if dpkg -i "$deb"; then
      echo "installed: /usr/bin/f"
      return 0
    fi
  elif need sudo; then
    if sudo dpkg -i "$deb"; then
      echo "installed: /usr/bin/f"
      return 0
    fi
  fi
  return 1
}

install_user_binary() {
  archive="$tmpdir/f.tar.gz"
  extract="$tmpdir/extract"
  url="$base/f_${version}_linux_${arch}.tar.gz"
  echo "downloading: $url"
  curl -fsSL "$url" -o "$archive"
  mkdir -p "$extract" "$BIN_DIR"
  tar -xzf "$archive" -C "$extract"
  binary="$(find "$extract" -type f -name f -print | sed -n '1p')"
  if [ -z "$binary" ]; then
    echo "error: release archive does not contain f" >&2
    exit 1
  fi
  install -m 0755 "$binary" "$BIN_DIR/f"
  echo "installed: $BIN_DIR/f"
  case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) echo "add to PATH: export PATH=\"$BIN_DIR:\$PATH\"" ;;
  esac
}

if need dpkg && { [ "$(id -u)" -eq 0 ] || need sudo; } && install_deb; then
  :
else
  install_user_binary
fi

if command -v f >/dev/null 2>&1; then
  f version
fi

echo "next: f setup"
