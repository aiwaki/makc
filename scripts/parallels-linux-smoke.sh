#!/usr/bin/env bash
set -euo pipefail

if ! command -v prlctl >/dev/null 2>&1; then
  echo "Parallels Desktop CLI (prlctl) is required" >&2
  exit 1
fi

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
vm="${MAKC_PARALLELS_LINUX_VM:-Fedora Linux (1)}"
run_user="${MAKC_PARALLELS_LINUX_USER:-root}"
timeout="${MAKC_PARALLELS_LINUX_TIMEOUT:-60}"

run_guest() {
  local user="$1"
  shift

  local user_args=()
  if [[ "$user" == "current" ]]; then
    user_args=(--current-user)
  else
    user_args=(--user "$user")
  fi

  if [[ "$timeout" == "0" ]]; then
    prlctl exec "$vm" "${user_args[@]}" "$@"
    return $?
  fi

  local timeout_flag
  timeout_flag="$(mktemp -t makc-parallels-linux-timeout.XXXXXX)"
  rm -f "$timeout_flag"

  prlctl exec "$vm" "${user_args[@]}" "$@" &
  local prlctl_pid=$!
  (
    sleep "$timeout"
    echo "prlctl exec timed out after ${timeout}s; set MAKC_PARALLELS_LINUX_TIMEOUT=0 to disable" >&2
    : >"$timeout_flag"
    kill -TERM "$prlctl_pid" >/dev/null 2>&1 || true
  ) &
  local watchdog_pid=$!

  set +e
  wait "$prlctl_pid"
  local status=$?
  set -e

  kill "$watchdog_pid" >/dev/null 2>&1 || true
  wait "$watchdog_pid" >/dev/null 2>&1 || true
  if [[ -f "$timeout_flag" ]]; then
    rm -f "$timeout_flag"
    return 124
  fi
  rm -f "$timeout_flag"
  return "$status"
}

prlctl resume "$vm" >/dev/null 2>&1 || true
status="$(prlctl status "$vm" 2>/dev/null || true)"
case "$status" in
  *" running")
    ;;
  *)
    echo "VM '$vm' is not running after resume: ${status:-unknown status}" >&2
    exit 1
    ;;
esac

guest_arch="$(run_guest current uname -m 2>/dev/null || true)"
case "$guest_arch" in
  aarch64|arm64)
    goarch=arm64
    ;;
  x86_64|amd64)
    goarch=amd64
    ;;
  "")
    echo "Could not run commands in '$vm'. Install Parallels Tools in the Linux guest first." >&2
    exit 1
    ;;
  *)
    echo "Unsupported Linux guest architecture: $guest_arch" >&2
    exit 1
    ;;
esac

host_stage="${MAKC_PARALLELS_LINUX_HOST_STAGE:-${MAKC_PARALLELS_LINUX_HOST_LINK:-$HOME/makc-parallels-smoke}}"
case "$host_stage" in
  "$HOME"/*)
    ;;
  *)
    echo "MAKC_PARALLELS_LINUX_HOST_STAGE must be under \$HOME" >&2
    exit 1
    ;;
esac
host_stage_name="$(basename "$host_stage")"
case "$host_stage_name" in
  *" "*)
    echo "MAKC_PARALLELS_LINUX_HOST_STAGE basename must not contain spaces: $host_stage_name" >&2
    exit 1
    ;;
esac
if [[ -L "$host_stage" ]]; then
  rm -f "$host_stage"
fi
mkdir -p "$host_stage/dist"
mkdir -p "$host_stage/scripts"
guest_repo="/media/psf/Home/$host_stage_name"

repo_visible=0
for _ in {1..3}; do
  if run_guest current test -d "$guest_repo"; then
    repo_visible=1
    break
  fi
  sleep 1
done

if [[ "$repo_visible" != "1" && "${MAKC_PARALLELS_LINUX_SHARE_HOME:-auto}" != "0" ]]; then
  echo "==> enable Parallels Home sharing"
  prlctl set "$vm" --shf-host on --shf-host-defined home --shf-host-automount on >/dev/null 2>&1 || true
  for _ in {1..20}; do
    if run_guest current test -d "$guest_repo"; then
      repo_visible=1
      break
    fi
    sleep 1
  done
fi

if [[ "$repo_visible" != "1" ]]; then
  cat >&2 <<EOF
The repo is not visible inside '$vm' at:
  $guest_repo

Enable Parallels host sharing for Home, or set MAKC_PARALLELS_LINUX_SHARE_HOME=1.
EOF
  exit 1
fi

host_exe="$host_stage/dist/makc-smoke-linux-$goarch"
guest_exe="$guest_repo/dist/makc-smoke-linux-$goarch"
host_portal_exe="$host_stage/dist/makc-portal-handshake-linux-$goarch"
guest_portal_exe="$guest_repo/dist/makc-portal-handshake-linux-$goarch"
guest_session_env="$guest_repo/scripts/linux-session-env.sh"
guest_gnome_remote_desktop_info="$guest_repo/scripts/linux-gnome-remote-desktop-info.sh"

echo "==> build linux/$goarch smoke binary"
GOOS=linux GOARCH="$goarch" go build -o "$host_exe" "$repo/cmd/makc-smoke"
GOOS=linux GOARCH="$goarch" go build -o "$host_portal_exe" "$repo/cmd/makc-portal-handshake"
cp "$repo/scripts/linux-session-env.sh" "$host_stage/scripts/linux-session-env.sh"
cp "$repo/scripts/linux-portal-info.sh" "$host_stage/scripts/linux-portal-info.sh"
cp "$repo/scripts/linux-gnome-remote-desktop-info.sh" "$host_stage/scripts/linux-gnome-remote-desktop-info.sh"
chmod 755 "$host_stage/scripts/linux-session-env.sh"
chmod 755 "$host_stage/scripts/linux-portal-info.sh"
chmod 755 "$host_stage/scripts/linux-gnome-remote-desktop-info.sh"

echo "==> prepare /dev/uinput"
run_guest root modprobe uinput || true
run_guest root test -e /dev/uinput
run_guest root ls -l /dev/uinput

echo "==> copy smoke binary into guest /tmp"
run_guest "$run_user" cp "$guest_exe" /tmp/makc-smoke
run_guest "$run_user" chmod 755 /tmp/makc-smoke
run_guest "$run_user" cp "$guest_portal_exe" /tmp/makc-portal-handshake
run_guest "$run_user" chmod 755 /tmp/makc-portal-handshake

if [[ "${MAKC_PARALLELS_LINUX_SESSION_DISCOVERY:-1}" != "0" ]]; then
  echo "==> discover active Linux GUI session"
  if ! run_guest current bash "$guest_session_env"; then
    echo "Linux GUI session discovery failed; continuing with headless smoke" >&2
  else
    echo "==> /tmp/makc-smoke -runtime-info with discovered session environment"
    run_guest current bash "$guest_session_env" --exec /tmp/makc-smoke -runtime-info || true
    echo "==> XDG Desktop Portal RemoteDesktop info"
    run_guest current bash "$guest_session_env" --exec bash "$guest_repo/scripts/linux-portal-info.sh" || true
    echo "==> XDG Desktop Portal RemoteDesktop CreateSession handshake"
    run_guest current bash "$guest_session_env" --exec /tmp/makc-portal-handshake || true
    if [[ "${MAKC_PARALLELS_LINUX_GNOME_REMOTE_DESKTOP_INFO:-1}" != "0" ]]; then
      echo "==> GNOME RemoteDesktop/Mutter info"
      run_guest current bash "$guest_session_env" --exec bash "$guest_gnome_remote_desktop_info" || true
    fi
  fi
fi

run_smoke() {
  echo "==> /tmp/makc-smoke $*"
  run_guest "$run_user" /tmp/makc-smoke "$@"
}

if [[ "$#" -gt 0 ]]; then
  run_smoke "$@"
else
  run_smoke -backend uinput -keyboard-backend uinput -capabilities -listen-backend evdev
	run_smoke -backend uinput -keyboard-backend uinput -inject -dx 1 -dy 1 -tap shift
	run_smoke -backend uinput -keyboard-backend uinput -scan 42
	run_smoke -backend uinput -keyboard-backend uinput -wheel 1 -hwheel 1
	run_smoke -backend uinput -keyboard-backend uinput -listen -listen-backend evdev -include-injected -listen-count 2
fi
