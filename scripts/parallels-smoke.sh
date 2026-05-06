#!/usr/bin/env bash
set -euo pipefail

vm="${MAKC_PARALLELS_VM:-Windows 11}"
timeout="${MAKC_PARALLELS_TIMEOUT:-60}"
repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
exe="dist/makc-smoke-windows-arm64.exe"

case "$repo" in
  "$HOME"/*)
    guest_repo="C:\\Mac\\Home${repo#"$HOME"}"
    guest_repo="${guest_repo//\//\\}"
    ;;
  *)
    echo "repo must be under \$HOME so Parallels can access it through C:\\Mac\\Home" >&2
    exit 1
    ;;
esac

mkdir -p "$repo/dist"
GOOS=windows GOARCH=arm64 go build -o "$repo/$exe" "$repo/cmd/makc-smoke"

prlctl resume "$vm" >/dev/null 2>&1 || true
sleep 1
status="$(prlctl status "$vm" 2>/dev/null || true)"
case "$status" in
  *" running")
    ;;
  *)
    echo "VM '$vm' is not running after resume: ${status:-unknown status}" >&2
    echo "If Parallels pauses it immediately, disable VM option 'Pause Windows when possible' / pause-idle." >&2
    exit 1
    ;;
esac

if [[ "$timeout" == "0" ]]; then
  prlctl exec "$vm" --current-user "$guest_repo\\$exe" "$@"
  exit $?
fi

timeout_flag="$(mktemp -t makc-parallels-timeout.XXXXXX)"
rm -f "$timeout_flag"
trap 'rm -f "$timeout_flag"' EXIT

prlctl exec "$vm" --current-user "$guest_repo\\$exe" "$@" &
prlctl_pid=$!
(
  sleep "$timeout"
  echo "prlctl exec timed out after ${timeout}s; set MAKC_PARALLELS_TIMEOUT=0 to disable" >&2
  : >"$timeout_flag"
  kill -TERM "$prlctl_pid" >/dev/null 2>&1 || true
) &
watchdog_pid=$!

set +e
wait "$prlctl_pid"
status=$?
set -e

kill "$watchdog_pid" >/dev/null 2>&1 || true
wait "$watchdog_pid" >/dev/null 2>&1 || true
if [[ -f "$timeout_flag" ]]; then
  status=124
fi
exit "$status"
