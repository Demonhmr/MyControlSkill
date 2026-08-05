# Развёртывание

Сервис — один статический бинарник и файл SQLite рядом. Ни базы данных
отдельным сервером, ни рантайма ставить не нужно.

## Что где лежит

```
/opt/mycontrolskill/bin/mycontrolskill-server   бинарник
/opt/mycontrolskill/bin/backup.sh               скрипт копии
/opt/mycontrolskill/web/                        сборка фронтенда
/var/lib/mycontrolskill/                        база (создаёт systemd)
/var/backups/mycontrolskill/                    копии
/etc/mycontrolskill/env                         настройки и секреты, 0600
```

## Сборка

На машине с Go и Node:

```bash
./scripts/build.sh server   # → build/mycontrolskill-server
npm --prefix app run build  # → app/dist
```

Бинарник статический (`CGO_ENABLED=0`), библиотек на сервере не требует.

## Установка

```bash
# Пользователь без входа в систему и без домашнего каталога.
sudo useradd --system --no-create-home --shell /usr/sbin/nologin mycontrolskill

sudo install -D -m 0755 build/mycontrolskill-server /opt/mycontrolskill/bin/mycontrolskill-server
sudo install -D -m 0755 deploy/backup.sh            /opt/mycontrolskill/bin/backup.sh
sudo mkdir -p /opt/mycontrolskill/web
sudo cp -r app/dist/. /opt/mycontrolskill/web/

sudo install -D -m 0600 deploy/env.example /etc/mycontrolskill/env
sudo nano /etc/mycontrolskill/env      # домен, почта, пароль

sudo cp deploy/mycontrolskill.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now mycontrolskill
```

Проверка:

```bash
curl -s localhost:8080/healthz          # {"status":"ok"}
journalctl -u mycontrolskill -n 30
```

В логе при старте видно, настроена ли почта. Строка «MCS_SMTP_HOST не
задан» означает, что ссылки уходят в лог, и войти сможет только тот, у кого
есть доступ к журналу.

## Обратный прокси

Сервис слушает петлю и не умеет TLS — наружу его выставляет прокси.

**Caddy** (проще: сертификат получает и продлевает сам):

```bash
sudo cp deploy/Caddyfile /etc/caddy/Caddyfile
sudo nano /etc/caddy/Caddyfile          # заменить домен
sudo systemctl reload caddy
```

**nginx** (если он уже стоит; сертификат через certbot отдельно):

```bash
sudo cp deploy/nginx.conf /etc/nginx/sites-available/mycontrolskill
sudo ln -s /etc/nginx/sites-available/mycontrolskill /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

Домен в конфигурации прокси и `MCS_BASE_URL` должны совпадать: `MCS_BASE_URL`
попадает в ссылки писем, а по его схеме включается флаг `Secure` у cookie
сессии.

`MCS_TRUST_PROXY=true` включать **только** когда прокси действительно стоит.
Без него все запросы выглядят пришедшими с адреса прокси и ограничение
частоты по источнику становится бессмысленным; с ним, но без прокси,
заголовок присылает сам клиент и ограничение обходится подделкой.

## Копии базы

```bash
sudo cp deploy/mycontrolskill-backup.{service,timer} /etc/systemd/system/
sudo install -d -o mycontrolskill -g mycontrolskill /var/backups/mycontrolskill
sudo systemctl daemon-reload
sudo systemctl enable --now mycontrolskill-backup.timer

systemctl list-timers mycontrolskill-backup   # когда сработает
sudo systemctl start mycontrolskill-backup    # проверить прямо сейчас
```

Копия делается самим бинарником (`-backup`), а не копированием файла. При
включённом WAL свежие изменения лежат в отдельном журнале рядом: на живой
базе файл `.db` может весить единицы килобайт, тогда как `.db-wal` — сотни,
и копия одного лишь `.db` потеряла бы почти всё. Служба во время копии
продолжает работать.

Копии старше 30 дней удаляются (`MCS_BACKUP_KEEP_DAYS`).

**Восстановление** — остановить службу, положить копию на место базы,
запустить:

```bash
sudo systemctl stop mycontrolskill
sudo cp /var/backups/mycontrolskill/mycontrolskill-2026-08-05T03-00-00.db \
        /var/lib/mycontrolskill/mycontrolskill.db
sudo rm -f /var/lib/mycontrolskill/mycontrolskill.db-wal \
           /var/lib/mycontrolskill/mycontrolskill.db-shm
sudo chown mycontrolskill:mycontrolskill /var/lib/mycontrolskill/mycontrolskill.db
sudo systemctl start mycontrolskill
```

Журналы `-wal` и `-shm` удаляются намеренно: они относятся к прежней базе, и
рядом с восстановленной означали бы несогласованное состояние.

## Обновление

```bash
sudo systemctl stop mycontrolskill
sudo install -m 0755 build/mycontrolskill-server /opt/mycontrolskill/bin/mycontrolskill-server
sudo rm -rf /opt/mycontrolskill/web && sudo mkdir -p /opt/mycontrolskill/web
sudo cp -r app/dist/. /opt/mycontrolskill/web/
sudo systemctl start mycontrolskill
```

Миграции применяются при старте сами, повторный запуск безопасен. Копию
перед обновлением лучше сделать вручную: `sudo systemctl start
mycontrolskill-backup`.

Простой на время перезапуска — секунды. Обновления без простоя нет: база
файловая, второй экземпляр рядом с первым запускать нельзя.

## Чего здесь нет

- **Нескольких экземпляров.** SQLite рассчитан на одного писателя, а
  счётчики ограничения частоты живут в памяти процесса — у второго
  экземпляра они будут свои.
- **Мониторинга.** Есть `/healthz`, который отвечает 200 только при живой
  базе; поднимать поверх него оповещения — отдельная задача.
- **Автоматической проверки восстановления.** Сама процедура выше проверена
  вручную: база с данными, копия, удаление базы, разворот копии — раунды и
  сессии на месте. Но никто не проверяет это регулярно, и молчаливая порча
  копий обнаружится только в момент, когда они понадобятся.
- **Вывоза копий за пределы машины.** Копии лежат на том же диске, что и
  база: от потери диска они не спасают.
