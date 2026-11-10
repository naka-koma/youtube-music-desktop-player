#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="$ROOT/Scripts/Setup.ps1"
WINDOWS_SCRIPT="$SCRIPT"

if command -v cygpath >/dev/null 2>&1; then
  WINDOWS_SCRIPT="$(cygpath -w "$SCRIPT")"
elif command -v wslpath >/dev/null 2>&1; then
  WINDOWS_SCRIPT="$(wslpath -w "$SCRIPT")"
fi

if command -v pwsh.exe >/dev/null 2>&1; then
  exec pwsh.exe -NoProfile -ExecutionPolicy Bypass -File "$WINDOWS_SCRIPT" "$@"
elif command -v powershell.exe >/dev/null 2>&1; then
  exec powershell.exe -NoProfile -ExecutionPolicy Bypass -File "$WINDOWS_SCRIPT" "$@"
else
  echo "PowerShell was not found. On Windows, run setup.bat; on Git Bash or WSL, make PowerShell available on PATH." >&2
  exit 1
fi
