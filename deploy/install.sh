#!/usr/bin/env bash
# Установка и обновление службы «Компас руководителя».
#
#   sudo ./deploy/install.sh              — установить или обновить
#   MCS_DRY_RUN=1 ./deploy/install.sh     — показать план, ничего не делая
#
# Перед запуском нужно собрать артефакты:
#
#   ./scripts/build.sh server
#   npm --prefix app run build
#
# Скрипт можно запускать повторно: обновление сводится к пересборке и
# повторному запуску. Файл с настройками при этом не трогается — в нём
# пароль от почты.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

SERVICE_USER="${MCS_USER:-mycontrolskill}"
OPT_DIR="${MCS_OPT_DIR:-/opt/mycontrolskill}"
ENV_FILE="${MCS_ENV_FILE:-/etc/mycontrolskill/env}"
BACKUP_DIR="${MCS_BACKUP_DIR:-/var/backups/mycontrolskill}"
UNIT_DIR="${MCS_UNIT_DIR:-/etc/systemd/system}"
DRY_RUN="${MCS_DRY_RUN:-}"

# run выполняет команду или показывает её при сухом прогоне.
run() {
  if [[ -n "$DRY_RUN" ]]; then
    printf '  … %s\n' "$*"
  else
    "$@"
  fi
}

say() { printf '\n== %s\n' "$*"; }
warn() { printf '  ! %s\n' "$*" >&2; }
die() { printf '\nОшибка: %s\n' "$*" >&2; exit 1; }

# --- проверки до первого изменения в системе -------------------------------

if [[ -z "$DRY_RUN" && "$(id -u)" != "0" ]]; then
  die "нужны права root: sudo $0"
fi

BINARY="$ROOT/build/mycontrolskill-server"
STATIC="$ROOT/app/dist"

[[ -x "$BINARY" ]] || die "нет собранного сервера ($BINARY). Соберите: ./scripts/build.sh server"
[[ -f "$STATIC/index.html" ]] || die "нет сборки фронтенда ($STATIC). Соберите: npm --prefix app run build"
command -v systemctl >/dev/null || die "systemd не найден: этот скрипт рассчитан на systemd"

if [[ -n "$DRY_RUN" ]]; then
  say "СУХОЙ ПРОГОН: показываю, что было бы сделано"
fi

# --- пользователь ----------------------------------------------------------

say "Служебный пользователь: $SERVICE_USER"
if id "$SERVICE_USER" >/dev/null 2>&1; then
  printf '  уже есть\n'
else
  # Без домашнего каталога и без возможности войти в систему: службе это
  # не нужно, а лишний вход — лишняя поверхность.
  run useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER"
fi

# --- файлы -----------------------------------------------------------------

say "Бинарник и скрипты: $OPT_DIR/bin"
run install -D -m 0755 "$BINARY" "$OPT_DIR/bin/mycontrolskill-server"
run install -D -m 0755 "$ROOT/deploy/backup.sh" "$OPT_DIR/bin/backup.sh"

say "Сборка фронтенда: $OPT_DIR/web"
# Каталог очищается целиком: у файлов Vite хэш в имени, и старые версии
# копились бы без конца.
run rm -rf "$OPT_DIR/web"
run mkdir -p "$OPT_DIR/web"
run cp -r "$STATIC/." "$OPT_DIR/web/"

say "Каталог для копий базы: $BACKUP_DIR"
run install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0750 "$BACKUP_DIR"

# --- настройки -------------------------------------------------------------

FRESH_ENV=""
say "Настройки: $ENV_FILE"
if [[ -f "$ENV_FILE" ]]; then
  # Здесь пароль от почты и настройки домена: перезаписывать нельзя ни при
  # каком обновлении.
  printf '  уже есть, не трогаю\n'
else
  run install -D -m 0600 -o root -g root "$ROOT/deploy/env.example" "$ENV_FILE"
  FRESH_ENV=1
fi

# --- юниты -----------------------------------------------------------------

say "Юниты systemd: $UNIT_DIR"
for unit in mycontrolskill.service \
            mycontrolskill-backup.service mycontrolskill-backup.timer \
            mycontrolskill-purge.service mycontrolskill-purge.timer; do
  run install -m 0644 "$ROOT/deploy/$unit" "$UNIT_DIR/$unit"
done
run systemctl daemon-reload

# --- запуск ----------------------------------------------------------------

if [[ -n "$FRESH_ENV" ]]; then
  # Настройки только что скопированы из примера: там чужой домен и пароль
  # «замените». Запускать службу с ними — значит рассылать письма в никуда
  # и получить непонятную поломку вместо внятной остановки.
  say "Служба НЕ запущена: сначала отредактируйте настройки"
  cat <<INSTRUCTIONS

  1. Впишите свой домен, почту и пароль:
       sudo nano $ENV_FILE

  2. Запустите службу и таймеры:
       sudo systemctl enable --now mycontrolskill
       sudo systemctl enable --now mycontrolskill-backup.timer
       sudo systemctl enable --now mycontrolskill-purge.timer

  3. Поднимите обратный прокси (см. deploy/README.md) — служба слушает
     только петлю и не умеет TLS.

INSTRUCTIONS
else
  say "Перезапуск службы"
  run systemctl enable --now mycontrolskill-backup.timer
  run systemctl enable --now mycontrolskill-purge.timer
  run systemctl enable mycontrolskill
  run systemctl restart mycontrolskill

  if [[ -z "$DRY_RUN" ]]; then
    sleep 1
    if systemctl is-active --quiet mycontrolskill; then
      printf '  служба работает\n'
    else
      warn "служба не поднялась, смотрите: journalctl -u mycontrolskill -n 50"
      exit 1
    fi
  fi
fi

say "Готово"
printf '  Проверка:   curl -s localhost:8080/healthz\n'
printf '  Журнал:     journalctl -u mycontrolskill -n 30\n'
printf '  Копии:      systemctl list-timers mycontrolskill-backup\n\n'
