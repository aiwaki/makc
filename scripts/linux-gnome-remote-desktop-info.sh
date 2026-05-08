#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/linux-gnome-remote-desktop-info.sh

Print read-only GNOME RemoteDesktop, Mutter, and session-lock diagnostics for
the current Linux desktop session. The script queries loginctl, D-Bus,
gsettings, systemd user state, and recent user journal lines only; it does not
change settings, request portal permissions, or inject input.
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "linux-gnome-remote-desktop-info.sh must be run on Linux" >&2
  exit 1
fi

timeout="${MAKC_GNOME_REMOTE_DESKTOP_INFO_TIMEOUT:-3}"
journal_since="${MAKC_GNOME_REMOTE_DESKTOP_JOURNAL_SINCE:--15min}"
journal_lines="${MAKC_GNOME_REMOTE_DESKTOP_JOURNAL_LINES:-20}"

parse_dbus_scalar() {
  awk '{print $2}' <<<"$1"
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

sanitize_name() {
  tr '.-' '__' <<<"$1"
}

prop_value() {
  local session="$1"
  local prop="$2"
  loginctl show-session "$session" -p "$prop" --value 2>/dev/null || true
}

session_score() {
  local session="$1"
  local name="$2"
  local class="$3"
  local state="$4"
  local type="$5"
  local score=0
  local target_user="${MAKC_LINUX_SESSION_USER:-${USER:-}}"

  [[ -n "$target_user" && "$name" == "$target_user" ]] && score=$((score + 100))
  [[ "$class" == "user" ]] && score=$((score + 20))
  [[ "$state" == "active" ]] && score=$((score + 20))
  case "$type" in
    wayland|x11) score=$((score + 30)) ;;
  esac
  [[ -n "$session" ]] && echo "$score"
}

choose_session() {
  local best_session=""
  local best_score=-1
  local session name class state type score

  while read -r session _; do
    [[ -z "${session:-}" ]] && continue
    name="$(prop_value "$session" Name)"
    class="$(prop_value "$session" Class)"
    state="$(prop_value "$session" State)"
    type="$(prop_value "$session" Type)"
    score="$(session_score "$session" "$name" "$class" "$state" "$type")"
    if (( score > best_score )); then
      best_score="$score"
      best_session="$session"
    fi
  done < <(loginctl list-sessions --no-legend 2>/dev/null || true)

  [[ -n "$best_session" ]] && echo "$best_session"
}

print_session_info() {
  if ! command -v loginctl >/dev/null 2>&1; then
    echo "gnome_remote_desktop_loginctl=false"
    echo "gnome_remote_desktop_loginctl_error=loginctl is not available"
    return 0
  fi

  echo "gnome_remote_desktop_loginctl=true"

  local session_id="${MAKC_LINUX_SESSION_ID:-${XDG_SESSION_ID:-}}"
  if [[ -z "$session_id" ]]; then
    session_id="$(choose_session)"
  fi

  if [[ -z "$session_id" ]]; then
    echo "gnome_remote_desktop_session_available=false"
    return 0
  fi

  echo "gnome_remote_desktop_session_available=true"
  echo "gnome_remote_desktop_session_id=$session_id"
  echo "gnome_remote_desktop_session_user=$(prop_value "$session_id" Name)"
  echo "gnome_remote_desktop_session_type=$(prop_value "$session_id" Type)"
  echo "gnome_remote_desktop_session_class=$(prop_value "$session_id" Class)"
  echo "gnome_remote_desktop_session_state=$(prop_value "$session_id" State)"
  echo "gnome_remote_desktop_session_active=$(prop_value "$session_id" Active)"
  echo "gnome_remote_desktop_session_locked_hint=$(prop_value "$session_id" LockedHint)"
  echo "gnome_remote_desktop_session_idle_hint=$(prop_value "$session_id" IdleHint)"
  echo "gnome_remote_desktop_session_remote=$(prop_value "$session_id" Remote)"
  echo "gnome_remote_desktop_session_service=$(prop_value "$session_id" Service)"
  echo "gnome_remote_desktop_session_desktop=$(prop_value "$session_id" Desktop)"
}

print_service_info() {
  if ! command -v systemctl >/dev/null 2>&1; then
    echo "gnome_remote_desktop_systemctl=false"
    return 0
  fi

  echo "gnome_remote_desktop_systemctl=true"
  echo "gnome_remote_desktop_service_active=$(systemctl --user is-active gnome-remote-desktop.service 2>/dev/null || true)"
  echo "gnome_remote_desktop_service_enabled=$(systemctl --user is-enabled gnome-remote-desktop.service 2>/dev/null || true)"
  echo "gnome_remote_desktop_service_load_state=$(systemctl --user show gnome-remote-desktop.service -p LoadState --value 2>/dev/null || true)"
  echo "gnome_remote_desktop_service_active_state=$(systemctl --user show gnome-remote-desktop.service -p ActiveState --value 2>/dev/null || true)"
  echo "gnome_remote_desktop_service_sub_state=$(systemctl --user show gnome-remote-desktop.service -p SubState --value 2>/dev/null || true)"
  echo "gnome_remote_desktop_service_unit_file_state=$(systemctl --user show gnome-remote-desktop.service -p UnitFileState --value 2>/dev/null || true)"
}

print_bus_info() {
  if ! command -v busctl >/dev/null 2>&1; then
    echo "gnome_remote_desktop_busctl=false"
    return 0
  fi

  echo "gnome_remote_desktop_busctl=true"

  local bus_address="${DBUS_SESSION_BUS_ADDRESS:-}"
  if [[ -z "$bus_address" ]]; then
    echo "gnome_remote_desktop_session_bus=false"
    echo "gnome_remote_desktop_session_bus_error=DBUS_SESSION_BUS_ADDRESS is not set"
    return 0
  fi
  echo "gnome_remote_desktop_session_bus=true"
  echo "gnome_remote_desktop_bus_address=$bus_address"

  local name="org.gnome.Mutter.RemoteDesktop"
  local path="/org/gnome/Mutter/RemoteDesktop"
  local iface="org.gnome.Mutter.RemoteDesktop"
  if busctl --user "--timeout=${timeout}" status "$name" >/dev/null 2>&1; then
    echo "gnome_mutter_remote_desktop_available=true"
    local version_raw devices_raw version devices
    version_raw="$(busctl --user "--timeout=${timeout}" get-property "$name" "$path" "$iface" Version 2>/dev/null || true)"
    devices_raw="$(busctl --user "--timeout=${timeout}" get-property "$name" "$path" "$iface" SupportedDeviceTypes 2>/dev/null || true)"
    if [[ -n "$version_raw" ]]; then
      version="$(parse_dbus_scalar "$version_raw")"
      echo "gnome_mutter_remote_desktop_version=$version"
    fi
    if [[ -n "$devices_raw" ]]; then
      devices="$(parse_dbus_scalar "$devices_raw")"
      echo "gnome_mutter_remote_desktop_devices=$devices"
      print_bool "gnome_mutter_remote_desktop_keyboard" "$(( (devices & 1) != 0 ))"
      print_bool "gnome_mutter_remote_desktop_pointer" "$(( (devices & 2) != 0 ))"
      print_bool "gnome_mutter_remote_desktop_touchscreen" "$(( (devices & 4) != 0 ))"
    fi
  else
    echo "gnome_mutter_remote_desktop_available=false"
  fi

  if busctl --user "--timeout=${timeout}" status org.gnome.ScreenSaver >/dev/null 2>&1; then
    echo "gnome_screensaver_available=true"
    local active_raw active_time_raw
    active_raw="$(busctl --user "--timeout=${timeout}" call org.gnome.ScreenSaver /org/gnome/ScreenSaver org.gnome.ScreenSaver GetActive 2>/dev/null || true)"
    active_time_raw="$(busctl --user "--timeout=${timeout}" call org.gnome.ScreenSaver /org/gnome/ScreenSaver org.gnome.ScreenSaver GetActiveTime 2>/dev/null || true)"
    if [[ -n "$active_raw" ]]; then
      echo "gnome_screensaver_active=$(parse_dbus_scalar "$active_raw")"
    fi
    if [[ -n "$active_time_raw" ]]; then
      echo "gnome_screensaver_active_time=$(parse_dbus_scalar "$active_time_raw")"
    fi
  else
    echo "gnome_screensaver_available=false"
  fi
}

print_gsettings_info() {
  if ! command -v gsettings >/dev/null 2>&1; then
    echo "gnome_remote_desktop_gsettings=false"
    return 0
  fi

  echo "gnome_remote_desktop_gsettings=true"
  local schema
  for schema in \
    org.gnome.desktop.remote-desktop \
    org.gnome.desktop.remote-desktop.rdp \
    org.gnome.desktop.remote-desktop.rdp.headless \
    org.gnome.desktop.remote-desktop.vnc \
    org.gnome.desktop.remote-desktop.vnc.headless; do
    local schema_key
    schema_key="$(sanitize_name "$schema")"
    if gsettings list-schemas 2>/dev/null | grep -Fxq "$schema"; then
      echo "gsettings_${schema_key}_schema=true"
      while read -r schema_name key value; do
        [[ -n "${schema_name:-}" && -n "${key:-}" ]] || continue
        local key_name
        key_name="$(sanitize_name "$key")"
        echo "gsettings_${schema_key}_${key_name}=${value:-}"
      done < <(gsettings list-recursively "$schema" 2>/dev/null || true)
    else
      echo "gsettings_${schema_key}_schema=false"
    fi
  done
}

print_journal_info() {
  if ! command -v journalctl >/dev/null 2>&1; then
    echo "gnome_remote_desktop_journal_available=false"
    return 0
  fi

  echo "gnome_remote_desktop_journal_available=true"
  local output
  output="$(
    journalctl --user --since "$journal_since" --no-pager -n 200 2>/dev/null |
      awk 'tolower($0) ~ /(remote.?desktop|mutter|portal|session creation inhibited)/ {print}' |
      tail -n "$journal_lines" || true
  )"
  if [[ -z "$output" ]]; then
    echo "gnome_remote_desktop_journal_matches=false"
    return 0
  fi

  echo "gnome_remote_desktop_journal_matches=true"
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    echo "gnome_remote_desktop_journal=$line"
  done <<<"$output"
}

print_session_info
print_service_info
print_bus_info
print_gsettings_info
print_journal_info
