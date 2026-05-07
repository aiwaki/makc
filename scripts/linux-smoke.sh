#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Linux" ]]; then
  echo "linux-smoke.sh must be run on Linux" >&2
  exit 1
fi

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
exe="$repo/dist/makc-smoke-linux"

if ! command -v go >/dev/null 2>&1; then
  echo "Go is required. On Fedora: sudo dnf install -y golang" >&2
  exit 1
fi

if [[ ! -e /dev/uinput ]]; then
  if command -v sudo >/dev/null 2>&1; then
    sudo modprobe uinput || true
  else
    modprobe uinput || true
  fi
fi

if [[ ! -e /dev/uinput ]]; then
  cat >&2 <<'EOF'
/dev/uinput is not available.

Try:
  sudo modprobe uinput

For a persistent setup, add a udev rule such as:
  echo 'KERNEL=="uinput", MODE="0660", GROUP="input", OPTIONS+="static_node=uinput"' | sudo tee /etc/udev/rules.d/99-uinput.rules
  sudo usermod -aG input "$USER"
  sudo udevadm control --reload-rules
  sudo udevadm trigger

Then log out and back in so the new group membership applies.
EOF
  exit 1
fi

mkdir -p "$repo/dist"
go build -o "$exe" "$repo/cmd/makc-smoke"

runner=()
if [[ -r /dev/uinput && -w /dev/uinput ]]; then
  echo "==> uinput device registration"
  MAKC_TEST_UINPUT=1 go test "$repo" -run TestLinuxUInputDeviceOpenIntegration -count=1 -v
elif [[ "${MAKC_LINUX_SMOKE_SUDO:-auto}" != "0" ]] && command -v sudo >/dev/null 2>&1; then
  echo "==> /dev/uinput needs elevated permissions; running smoke binary with sudo"
  runner=(sudo env "PATH=$PATH")
else
  cat >&2 <<'EOF'
/dev/uinput exists, but the current user cannot read and write it.

Either run:
  sudo bash scripts/linux-smoke.sh

Or configure persistent permissions with:
  echo 'KERNEL=="uinput", MODE="0660", GROUP="input", OPTIONS+="static_node=uinput"' | sudo tee /etc/udev/rules.d/99-uinput.rules
  sudo usermod -aG input "$USER"
  sudo udevadm control --reload-rules
  sudo udevadm trigger

Then log out and back in.
EOF
  exit 1
fi

if [[ "$#" -eq 0 ]]; then
  set -- -backend uinput -keyboard-backend uinput -inject -dx 1 -dy 1 -tap shift
fi

echo "==> makc-smoke $*"
echo "This sends the requested input events to the active Linux session."
"${runner[@]}" "$exe" "$@"
