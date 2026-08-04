#!/usr/bin/env bash
# Собирает самодостаточный .exe с прототипом внутри.
#
# Порядок: сборка Vite → копирование статики в launcher/dist → go build.
# Статика вшивается в бинарник через go:embed, поэтому рядом с .exe
# ничего класть не нужно.
#
# Требуется Go (см. README, ставится распаковкой архива без sudo).
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(dirname "$HERE")"
OUT="$ROOT/build"

GOBIN="${GOROOT_LOCAL:-$HOME/.local/goroot}/bin/go"
[[ -x "$GOBIN" ]] || GOBIN="$(command -v go || true)"
if [[ -z "$GOBIN" || ! -x "$GOBIN" ]]; then
  echo "Go не найден. Установите его или задайте GOROOT_LOCAL." >&2
  exit 1
fi

echo "==> Сборка веб-приложения (Vite)"
cd "$ROOT/app"
[[ -d node_modules ]] || npm install --no-audit --no-fund
npm run build

echo "==> Перенос статики в лаунчер"
rm -rf "$HERE/dist"
cp -r "$ROOT/app/dist" "$HERE/dist"

echo "==> Сборка бинарников"
mkdir -p "$OUT"
cd "$HERE"

# CGO выключен — бинарник получается статическим, без зависимостей от
# системных библиотек, и одинаково работает на любой Windows 10/11 x64.
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 "$GOBIN" build \
  -trimpath -ldflags="-s -w" -o "$OUT/MyControlSkill.exe" .

# Linux-версия — для проверки на этой же машине.
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 "$GOBIN" build \
  -trimpath -ldflags="-s -w" -o "$OUT/MyControlSkill-linux" .

echo
echo "Готово:"
ls -lh "$OUT" | tail -n +2 | awk '{printf "  %-28s %s\n", $9, $5}'
