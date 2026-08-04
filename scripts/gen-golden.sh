#!/usr/bin/env bash
# Перегенерирует эталонные векторы скоринга из клиентской реализации.
#
# Через vite-node, а не через node: scoring.js импортирует '../data' —
# каталог, а не файл. Vite такой импорт разрешает, обычный Node — нет.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(dirname "$HERE")"

cd "$ROOT/app"
[[ -d node_modules ]] || npm install --no-audit --no-fund
exec node_modules/.bin/vite-node "$HERE/gen-golden.mjs"
