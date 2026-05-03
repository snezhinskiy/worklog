#!/usr/bin/env bash
# worklog installer — builds the binary and registers the MCP server.
#
# Usage:
#   bash install.sh --client claude-code
#   bash install.sh --client claude-desktop
#   bash install.sh --client none           # just build + install the binary
#
# Run from a fresh `git clone` of the worklog repo.

set -euo pipefail

CLIENT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --client) CLIENT="${2:-}"; shift 2 ;;
    --client=*) CLIENT="${1#*=}"; shift ;;
    -h|--help)
      sed -n '2,9p' "$0" | sed 's/^# \{0,1\}//'
      exit 0 ;;
    *) echo "install.sh: unknown arg: $1" >&2; exit 2 ;;
  esac
done

if [ -z "$CLIENT" ]; then
  echo "install.sh: --client is required (claude-code | claude-desktop | none)" >&2
  exit 2
fi

# --- 1. sanity checks ------------------------------------------------------

command -v go >/dev/null 2>&1 || {
  echo "install.sh: 'go' not found in PATH. Install Go 1.26+ first: https://go.dev/dl/" >&2
  exit 1
}

REPO_DIR="$(cd "$(dirname "$0")" && pwd)"
[ -f "$REPO_DIR/go.mod" ] || {
  echo "install.sh: must be run from the worklog repo root (no go.mod next to install.sh)" >&2
  exit 1
}

# --- 2. build + install ----------------------------------------------------

INSTALL_DIR="$(go env GOPATH)/bin"
mkdir -p "$INSTALL_DIR"
BIN="$INSTALL_DIR/worklog"

echo "==> Building worklog..."
( cd "$REPO_DIR" && go build -o "$BIN" ./cmd/worklog )
echo "    installed: $BIN"

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "    NOTE: $INSTALL_DIR is not in your PATH — add this to ~/.zshrc or ~/.bashrc:"
     echo "      export PATH=\"\$PATH:$INSTALL_DIR\"" ;;
esac

# --- 3. register MCP -------------------------------------------------------

case "$CLIENT" in
  none)
    echo "==> Skipping MCP registration (--client none)."
    ;;
  claude-code)
    echo "==> Registering with Claude Code (user scope)..."
    if ! command -v claude >/dev/null 2>&1; then
      echo "    'claude' CLI not found. Install Claude Code first, then run:"
      echo "      claude mcp add worklog -s user -- $BIN mcp"
      exit 1
    fi
    # Re-add idempotently: remove existing entry if present, ignore "not found".
    claude mcp remove worklog -s user >/dev/null 2>&1 || true
    claude mcp add worklog -s user -- "$BIN" mcp
    echo "    done. Verify with: claude mcp list"
    ;;
  claude-desktop)
    echo "==> Registering with Claude Desktop..."
    case "$(uname -s)" in
      Darwin)  CFG="$HOME/Library/Application Support/Claude/claude_desktop_config.json" ;;
      Linux)   CFG="${XDG_CONFIG_HOME:-$HOME/.config}/Claude/claude_desktop_config.json" ;;
      MINGW*|MSYS*|CYGWIN*) CFG="${APPDATA:-$HOME/AppData/Roaming}/Claude/claude_desktop_config.json" ;;
      *)       echo "    Unknown OS: $(uname -s). Edit your config manually." >&2; exit 1 ;;
    esac
    mkdir -p "$(dirname "$CFG")"
    [ -f "$CFG" ] || echo '{}' > "$CFG"
    cp "$CFG" "$CFG.bak.$(date +%Y%m%d%H%M%S)"

    command -v python3 >/dev/null 2>&1 || {
      echo "    'python3' not found — needed to patch the JSON config safely." >&2
      echo "    Add this entry manually under 'mcpServers' in: $CFG" >&2
      echo "      \"worklog\": { \"command\": \"$BIN\", \"args\": [\"mcp\"] }" >&2
      exit 1
    }
    BIN="$BIN" CFG="$CFG" python3 - <<'PY'
import json, os, sys
cfg_path = os.environ["CFG"]
bin_path = os.environ["BIN"]
with open(cfg_path) as f:
    try: data = json.load(f)
    except json.JSONDecodeError: data = {}
data.setdefault("mcpServers", {})["worklog"] = {"command": bin_path, "args": ["mcp"]}
with open(cfg_path, "w") as f:
    json.dump(data, f, indent=2)
    f.write("\n")
print(f"    patched: {cfg_path}")
PY
    echo "    done. Restart Claude Desktop to pick up the new server."
    ;;
  *)
    echo "install.sh: unknown --client '$CLIENT' (expected claude-code | claude-desktop | none)" >&2
    exit 2
    ;;
esac

echo "==> All set. Try: worklog"
