#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/linux-portal-info.sh

Print read-only XDG Desktop Portal RemoteDesktop diagnostics for the current
session bus. The script queries D-Bus properties only; it does not create a
portal session, request permissions, or inject input.
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "linux-portal-info.sh must be run on Linux" >&2
  exit 1
fi

if ! command -v busctl >/dev/null 2>&1; then
  echo "busctl is required" >&2
  exit 1
fi

bus_address="${DBUS_SESSION_BUS_ADDRESS:-}"
if [[ -z "$bus_address" ]]; then
  echo "portal_session_bus=false"
  echo "portal_error=DBUS_SESSION_BUS_ADDRESS is not set"
  exit 0
fi

portal_name="org.freedesktop.portal.Desktop"
portal_path="/org/freedesktop/portal/desktop"
remote_desktop_iface="org.freedesktop.portal.RemoteDesktop"
timeout="${MAKC_PORTAL_INFO_TIMEOUT:-3}"

get_property() {
  local property="$1"
  busctl --user "--timeout=${timeout}" get-property \
    "$portal_name" \
    "$portal_path" \
    "$remote_desktop_iface" \
    "$property" 2>/dev/null
}

parse_uint_property() {
  local raw="$1"
  awk '{print $2}' <<<"$raw"
}

print_bool() {
  local name="$1"
  local value="$2"
  if (( value )); then
    echo "$name=true"
  else
    echo "$name=false"
  fi
}

echo "portal_session_bus=true"
echo "portal_bus_address=$bus_address"

if ! busctl --user "--timeout=${timeout}" status "$portal_name" >/dev/null 2>&1; then
  echo "portal_desktop_available=false"
  echo "portal_error=$portal_name is not available on the session bus"
  exit 0
fi
echo "portal_desktop_available=true"

version_raw="$(get_property version || true)"
if [[ -n "$version_raw" ]]; then
  version="$(parse_uint_property "$version_raw")"
  echo "portal_remote_desktop_available=true"
  echo "portal_remote_desktop_version=$version"
else
  echo "portal_remote_desktop_available=false"
  echo "portal_remote_desktop_error=$remote_desktop_iface.version is not readable"
  exit 0
fi

devices_raw="$(get_property AvailableDeviceTypes || true)"
if [[ -z "$devices_raw" ]]; then
  echo "portal_remote_desktop_devices_error=$remote_desktop_iface.AvailableDeviceTypes is not readable"
  exit 0
fi

devices="$(parse_uint_property "$devices_raw")"
echo "portal_remote_desktop_devices=$devices"
print_bool "portal_remote_desktop_keyboard" "$(( (devices & 1) != 0 ))"
print_bool "portal_remote_desktop_pointer" "$(( (devices & 2) != 0 ))"
print_bool "portal_remote_desktop_touchscreen" "$(( (devices & 4) != 0 ))"
print_bool "portal_remote_desktop_connect_to_eis" "$(( version >= 2 ))"
