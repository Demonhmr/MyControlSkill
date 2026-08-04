#!/usr/bin/env bash
# Собирает бинарники проекта.
#
#   ./scripts/build.sh            — лаунчер (.exe + linux) и сервер
#   ./scripts/build.sh launcher   — только лаунчер
#   ./scripts/build.sh server     — только сервер
#
# Лаунчер: сборка Vite → копирование статики в cmd/launcher/dist → go build.
# Статика вшивается в бинарник через go:embed, поэтому рядом с .exe ничего
# класть не нужно.
#
# Сервер статику не вшивает: она отдаётся с диска из каталога MCS_STATIC_DIR,
# чтобы фронт можно было выкатывать отдельно от бэкенда.
#
# Требуется Go (см. README, ставится распаковкой архива без sudo).
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(dirname "$HERE")"
OUT="$ROOT/build"

TARGET="${1:-all}"
case "$TARGET" in
  all|launcher|server) ;;
  *) echo "Неизвестная цель: $TARGET (ожидается all, launcher или server)" >&2; exit 1 ;;
esac

GOBIN="${GOROOT_LOCAL:-$HOME/.local/goroot}/bin/go"
[[ -x "$GOBIN" ]] || GOBIN="$(command -v go || true)"
if [[ -z "$GOBIN" || ! -x "$GOBIN" ]]; then
  echo "Go не найден. Установите его или задайте GOROOT_LOCAL." >&2
  exit 1
fi

mkdir -p "$OUT"

build_frontend() {
  echo "==> Сборка веб-приложения (Vite)"
  cd "$ROOT/app"
  [[ -d node_modules ]] || npm install --no-audit --no-fund
  npm run build
}

if [[ "$TARGET" == "all" || "$TARGET" == "launcher" ]]; then
  build_frontend

  echo "==> Перенос статики в лаунчер"
  # Каталог не удаляем: в нём лежит .gitkeep, без которого go:embed
  # не компилируется на чистом клоне. Чистим только содержимое.
  find "$ROOT/cmd/launcher/dist" -mindepth 1 ! -name .gitkeep -delete
  cp -r "$ROOT/app/dist/." "$ROOT/cmd/launcher/dist/"

  echo "==> Сборка лаунчера"
  cd "$ROOT"
  # CGO выключен — бинарник получается статическим, без зависимостей от
  # системных библиотек, и одинаково работает на любой Windows 10/11 x64.
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 "$GOBIN" build \
    -trimpath -ldflags="-s -w" -o "$OUT/MyControlSkill.exe" ./cmd/launcher

  # Linux-версия — для проверки на этой же машине.
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 "$GOBIN" build \
    -trimpath -ldflags="-s -w" -o "$OUT/MyControlSkill-linux" ./cmd/launcher
fi

if [[ "$TARGET" == "all" || "$TARGET" == "server" ]]; then
  echo "==> Сборка сервера"
  cd "$ROOT"
  # Драйвер SQLite — modernc.org/sqlite, чистый Go, поэтому CGO не нужен
  # и здесь: сервер тоже остаётся одним статическим файлом.
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 "$GOBIN" build \
    -trimpath -ldflags="-s -w" -o "$OUT/mycontrolskill-server" ./cmd/server
fi

echo
echo "Готово:"
ls -lh "$OUT" | tail -n +2 | awk '{printf "  %-28s %s\n", $9, $5}'
