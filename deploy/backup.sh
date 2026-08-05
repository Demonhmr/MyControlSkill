#!/usr/bin/env bash
# Ежедневная копия базы. Запускается таймером systemd.
#
# Копия делается самим сервером (VACUUM INTO), а не копированием файла: при
# включённом WAL часть свежих изменений лежит в отдельном журнале, и копия
# одного лишь .db может оказаться без них или вовсе битой.
#
# Служба при этом продолжает работать — SQLite допускает чтение из другого
# процесса.
set -euo pipefail

BIN="${MCS_BIN:-/opt/mycontrolskill/bin/mycontrolskill-server}"
DEST_DIR="${MCS_BACKUP_DIR:-/var/backups/mycontrolskill}"
KEEP_DAYS="${MCS_BACKUP_KEEP_DAYS:-30}"

mkdir -p "$DEST_DIR"

# Имя со временем, а не только с датой: повторный запуск в тот же день не
# должен падать на «файл уже существует».
DEST="$DEST_DIR/mycontrolskill-$(date +%Y-%m-%dT%H-%M-%S).db"

"$BIN" -backup "$DEST"
chmod 0600 "$DEST"

# Старые копии удаляются, иначе диск кончится незаметно.
find "$DEST_DIR" -name 'mycontrolskill-*.db' -type f -mtime "+$KEEP_DAYS" -delete

echo "Копия готова: $DEST"
