#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/linux-session-env.sh [--shell | --exec COMMAND [ARG...]]

Discover the active Linux graphical session and the environment variables
needed by display-server and portal-aware tools. The script only reads
loginctl and /proc metadata; it does not inject input or interact with the UI.

Modes:
  default          print a human-readable session summary
  --shell         print shell exports for the discovered environment
  --exec COMMAND  run COMMAND with the discovered environment
EOF
}

mode="summary"
if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
elif [[ "${1:-}" == "--shell" ]]; then
  mode="shell"
  shift
elif [[ "${1:-}" == "--exec" ]]; then
  mode="exec"
  shift
  if [[ "$#" -eq 0 ]]; then
    echo "--exec requires a command" >&2
    exit 2
  fi
fi

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "linux-session-env.sh must be run on Linux" >&2
  exit 1
fi

if ! command -v loginctl >/dev/null 2>&1; then
  echo "loginctl is required" >&2
  exit 1
fi

target_user="${MAKC_LINUX_SESSION_USER:-${SUDO_USER:-${USER:-}}}"
if [[ -z "$target_user" ]] && command -v id >/dev/null 2>&1; then
  target_user="$(id -un 2>/dev/null || true)"
fi

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

  [[ -n "$target_user" && "$name" == "$target_user" ]] && score=$((score + 100))
  [[ "$class" == "user" ]] && score=$((score + 20))
  [[ "$state" == "active" ]] && score=$((score + 20))
  case "$type" in
    wayland|x11) score=$((score + 30)) ;;
    tty) score=$((score + 5)) ;;
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

session_id="${MAKC_LINUX_SESSION_ID:-$(choose_session)}"
if [[ -z "$session_id" ]]; then
  echo "No loginctl session found" >&2
  exit 1
fi

session_uid="$(prop_value "$session_id" User)"
session_name="$(prop_value "$session_id" Name)"
session_type="$(prop_value "$session_id" Type)"
session_class="$(prop_value "$session_id" Class)"
session_state="$(prop_value "$session_id" State)"
session_leader="$(prop_value "$session_id" Leader)"
session_service="$(prop_value "$session_id" Service)"
session_desktop="$(prop_value "$session_id" Desktop)"

declare -A env_map=()

set_env() {
  local key="$1"
  local value="$2"
  [[ -n "$key" && -n "$value" && -z "${env_map[$key]:-}" ]] || return 0
  env_map["$key"]="$value"
}

read_proc_env() {
  local pid="$1"
  [[ -n "$pid" && -r "/proc/$pid/environ" ]] || return 0
  while IFS='=' read -r key value; do
    case "$key" in
      DISPLAY|WAYLAND_DISPLAY|XDG_SESSION_ID|XDG_SESSION_TYPE|XDG_CURRENT_DESKTOP|XDG_RUNTIME_DIR|DESKTOP_SESSION|DBUS_SESSION_BUS_ADDRESS)
        set_env "$key" "$value"
        ;;
    esac
  done < <(tr '\0' '\n' <"/proc/$pid/environ" 2>/dev/null || true)
}

add_pids_by_pattern() {
  local pattern="$1"
  pgrep -u "$session_uid" -f "$pattern" 2>/dev/null || true
}

pid_seen=" "
read_pid_once() {
  local pid="$1"
  [[ -n "$pid" ]] || return 0
  case "$pid_seen" in
    *" $pid "*) return 0 ;;
  esac
  pid_seen+="$pid "
  read_proc_env "$pid"
}

read_pid_once "$session_leader"
while read -r pid; do
  read_pid_once "$pid"
done < <(add_pids_by_pattern '(^|/)(gnome-session|gnome-shell|kwin_wayland|plasmashell|Xorg|Xwayland|xdg-desktop-portal|dbus-daemon|dbus-broker)( |$)')

env_map["XDG_SESSION_ID"]="$session_id"
if [[ -n "$session_type" && "$session_type" != "unspecified" ]]; then
  set_env XDG_SESSION_TYPE "$session_type"
fi
if [[ -n "$session_desktop" ]]; then
  set_env XDG_CURRENT_DESKTOP "$session_desktop"
fi
if [[ -n "$session_uid" && -d "/run/user/$session_uid" ]]; then
  set_env XDG_RUNTIME_DIR "/run/user/$session_uid"
fi
if [[ -n "${env_map[XDG_RUNTIME_DIR]:-}" && -S "${env_map[XDG_RUNTIME_DIR]}/bus" ]]; then
  set_env DBUS_SESSION_BUS_ADDRESS "unix:path=${env_map[XDG_RUNTIME_DIR]}/bus"
fi
if [[ -z "${env_map[WAYLAND_DISPLAY]:-}" && -n "${env_map[XDG_RUNTIME_DIR]:-}" ]]; then
  while IFS= read -r socket; do
    set_env WAYLAND_DISPLAY "$(basename "$socket")"
    break
  done < <(find "${env_map[XDG_RUNTIME_DIR]}" -maxdepth 1 -type s -name 'wayland-*' 2>/dev/null | sort)
fi
if [[ -z "${env_map[DISPLAY]:-}" ]]; then
  while IFS= read -r socket; do
    set_env DISPLAY ":${socket##*X}"
    break
  done < <(find /tmp/.X11-unix -maxdepth 1 -type s -name 'X*' 2>/dev/null | sort)
fi

emit_shell() {
  local key
  for key in DISPLAY WAYLAND_DISPLAY XDG_SESSION_ID XDG_SESSION_TYPE XDG_CURRENT_DESKTOP XDG_RUNTIME_DIR DESKTOP_SESSION DBUS_SESSION_BUS_ADDRESS; do
    [[ -n "${env_map[$key]:-}" ]] || continue
    printf 'export %s=%q\n' "$key" "${env_map[$key]}"
  done
}

case "$mode" in
  shell)
    emit_shell
    ;;
  exec)
    env_args=()
    for key in DISPLAY WAYLAND_DISPLAY XDG_SESSION_ID XDG_SESSION_TYPE XDG_CURRENT_DESKTOP XDG_RUNTIME_DIR DESKTOP_SESSION DBUS_SESSION_BUS_ADDRESS; do
      [[ -n "${env_map[$key]:-}" ]] || continue
      env_args+=("$key=${env_map[$key]}")
    done
    exec env "${env_args[@]}" "$@"
    ;;
  summary)
    echo "session_id=$session_id"
    echo "session_user=$session_name"
    echo "session_uid=$session_uid"
    echo "session_type=$session_type"
    echo "session_class=$session_class"
    echo "session_state=$session_state"
    echo "session_leader=$session_leader"
    echo "session_service=$session_service"
    [[ -n "$session_desktop" ]] && echo "session_desktop=$session_desktop"
    for key in DISPLAY WAYLAND_DISPLAY XDG_SESSION_ID XDG_SESSION_TYPE XDG_CURRENT_DESKTOP XDG_RUNTIME_DIR DESKTOP_SESSION DBUS_SESSION_BUS_ADDRESS; do
      [[ -n "${env_map[$key]:-}" ]] || continue
      echo "env_$key=${env_map[$key]}"
    done
    ;;
esac
