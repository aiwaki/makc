#!/usr/bin/env bash
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$repo"

echo "==> gofmt"
gofmt_out="$(find . -name '*.go' -not -path './.git/*' -not -path './dist/*' -print | sort | xargs gofmt -l)"
if [[ -n "$gofmt_out" ]]; then
  echo "$gofmt_out"
  echo "Run gofmt on the files above." >&2
  exit 1
fi

echo "==> go test"
go test ./...

echo "==> windows arm64 compile tests"
GOOS=windows GOARCH=arm64 go test -exec=true ./...

echo "==> darwin arm64 compile tests"
GOOS=darwin GOARCH=arm64 go test -exec=true ./...

echo "==> linux arm64 compile tests"
GOOS=linux GOARCH=arm64 go test -exec=true ./...

echo "==> go vet"
go vet ./...

echo "==> windows arm64 vet"
GOOS=windows GOARCH=arm64 go vet ./...

echo "==> darwin arm64 vet"
GOOS=darwin GOARCH=arm64 go vet ./...

echo "==> linux arm64 vet"
GOOS=linux GOARCH=arm64 go vet ./...

if [[ "${MAKC_CHECK_PARALLELS:-0}" == "1" ]]; then
  echo "==> parallels smoke: backend open"
  bash scripts/parallels-smoke.sh -backend auto -keyboard-backend auto

  echo "==> parallels smoke: inject backends"
  bash scripts/parallels-smoke.sh -backend injectmouseinput -keyboard-backend injectkeyboardinput -inject -dx 1 -dy 1

  echo "==> parallels smoke: sendinput tagging"
  bash scripts/parallels-smoke.sh -backend sendinput -keyboard-backend sendinput -input-tag 0x1234 -listen -normalize-own-injected -listen-count 3
fi
