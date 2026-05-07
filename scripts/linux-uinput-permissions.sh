#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "linux-uinput-permissions.sh must be run on Linux" >&2
  exit 1
fi

if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  if ! command -v sudo >/dev/null 2>&1; then
    echo "root or sudo is required to configure /dev/uinput permissions" >&2
    exit 1
  fi
  exec sudo bash "$0" "$@"
fi

group="${MAKC_UINPUT_GROUP:-input}"
target_user="${1:-${MAKC_UINPUT_USER:-${SUDO_USER:-}}}"
rule="/etc/udev/rules.d/99-makc-uinput.rules"

if ! getent group "$group" >/dev/null 2>&1; then
  groupadd --system "$group"
fi

cat >"$rule" <<EOF
KERNEL=="uinput", MODE="0660", GROUP="$group", OPTIONS+="static_node=uinput"
EOF

modprobe uinput || true
udevadm control --reload-rules || true
udevadm trigger --name-match=uinput || true

if [[ -e /dev/uinput ]]; then
  chgrp "$group" /dev/uinput
  chmod 660 /dev/uinput
fi

if [[ -n "$target_user" ]]; then
  usermod -aG "$group" "$target_user"
  echo "Added $target_user to group $group."
  echo "Log out and back in, or restart the VM, so the group membership applies."
else
  echo "No user was provided. Run again as: sudo bash scripts/linux-uinput-permissions.sh <user>"
fi

echo "Installed $rule."
ls -l /dev/uinput 2>/dev/null || true
