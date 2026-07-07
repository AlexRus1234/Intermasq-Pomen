# Установка и настройка Pomen

## 1. Требования

- Работающий **Intermasq** с поддержкой плагинов (`/etc/intermasq/plugins/`).
- **Caddy** на каждой ноде с настроенным Admin API (`:2019`) и Step-CA (см. документацию Povez).
- **Wildcard DNS** в dnsmasq для каждой ноды: `address=/.yadr00.internal/172.20.5.3` (IP Caddy ноды).
- **Вебхук adnanh/webhook** на каждой ВМ с контейнерами Podman (порт 9000 или любой).
- Podman запущен от того же пользователя, что и вебхук.

## 2. Установка плагина на сервер Intermasq

### 2.1 Перенос файлов
```bash
sudo mkdir -p /etc/intermasq/plugins/pomen
sudo cp pomen-linux                /etc/intermasq/plugins/pomen/pomen
sudo chmod +x                      /etc/intermasq/plugins/pomen/pomen
sudo cp config.json index.html manifest.json /etc/intermasq/plugins/pomen/
sudo chown -R intermasq:intermasq  /etc/intermasq/plugins/pomen
```

### 2.2 config.json (статика — ноды и их Caddy)
`/etc/intermasq/plugins/pomen/config.json`:
```json
{
    "base_domain": ".internal",
    "nodes": {
        "yadr00": { "caddy_url": "http://172.20.5.3:2019" },
        "yadr01": { "caddy_url": "http://172.20.6.3:2019" }
    }
}
```
- `base_domain` — суффикс всех доменов. FQDN = `<container>.<vm>.<node><base_domain>`.
- `nodes` — map нод на URL их Caddy Admin API. **Те же, что в Povez.**

Секретов ВМ тут **нет** — они в `vms.json`, добавляются через UI.

### 2.3 manifest.json
Уже в комплекте:
```json
{
    "id": "pomen",
    "name": "Pomen",
    "bin": "pomen"
}
```
Intermasq при старте сам:
- найдёт `/etc/intermasq/plugins/pomen/manifest.json`
- запустит `pomen` как дочерний процесс
- передаст `PLUGIN_SOCKET=/run/intermasq/sockets/pomen.sock`
- проксирует `/plugins/pomen/*` на этот сокет

### 2.4 Перезапуск Intermasq
```bash
sudo systemctl restart intermasq
sudo journalctl -u intermasq --since "1 min ago" | grep PLUGINS
```
Должно появиться: `[PLUGINS] Started Pomen on socket /run/intermasq/sockets/pomen.sock`.

### 2.5 Доступ к UI
```
http://<IP_INTERMASQ>:8080/plugins/pomen/
```

## 3. Настройка вебхука на ВМ

На каждой ВМ с Podman должен быть запущен `adnanh/webhook` с хуком `podman`.

### 3.1 hooks-файл для Pomen
Создай отдельный файл (не мешая боевым хукам git-sync):
```bash
cat > /opt/appdata/config/webhook/pomen-hooks.json <<'EOF'
[
  {
    "id": "podman",
    "execute-command": "/usr/bin/podman",
    "pass-arguments-to-command": [
      { "source": "string", "name": "ps" },
      { "source": "string", "name": "--format" },
      { "source": "string", "name": "json" }
    ],
    "command-working-directory": "/home/app-runner",
    "capture-command-output": true,
    "include-command-output-in-response": true,
    "include-command-output-in-response-on-error": true,
    "trigger-rule": {
      "match": {
        "type": "value",
        "value": "ТВОЙ_СЕКРЕТ",
        "parameter": { "source": "header", "name": "X-VM-Secret" }
      }
    }
  }
]
EOF
```

### 3.2 Подключение к запуску вебхука
Если вебхук уже запущен с одним hooks-файлом — добавь второй через override:
```bash
mkdir -p ~/.config/systemd/user/webhook.service.d
cat > ~/.config/systemd/user/webhook.service.d/override.conf <<'EOF'
[Service]
ExecStart=
ExecStart=%h/.local/bin/webhook -hooks /opt/appdata/config/webhook/hooks.json -hooks /opt/appdata/config/webhook/pomen-hooks.json -verbose -port 9000
EOF

systemctl --user daemon-reload
systemctl --user restart webhook
systemctl --user status webhook
```

### 3.3 Проверка
С сервера Intermasq:
```bash
curl -X POST -H 'X-VM-Secret: ТВОЙ_СЕКРЕТ' -H 'Content-Type: application/json' -d '{}' http://IP_VM:9000/hooks/podman
```
Должен вернуться JSON-массив контейнеров от `podman ps --format json`.

> **Важно:** в curl используй **одинарные кавычки** для заголовка с секретом, если в секрете есть `$` — иначе bash раскроет его как переменную.

## 4. Регистрация ВМ в UI Pomen

1. Открой UI: `http://<IP_INTERMASQ>:8080/plugins/pomen/`
2. Вкладка **«Виртуальные машины»** → форма «Добавить ВМ»:

| Поле | Пример | Описание |
|---|---|---|
| Имя | `obshaga` | Уникальное имя ВМ (регистронезависимое) |
| Нода | `yadr00` | Дропдаун из config.json |
| IP | `172.20.5.17` | IP ВМ в сети |
| Webhook URL | `http://172.20.5.17:9000` | URL вебхука ВМ |
| Секрет | `nP$JANUTx^#39dKzs3LQ` | Значение `value` из pomen-hooks.json |

3. Жми «Добавить». Запись попадёт в `/etc/intermasq/plugins/pomen/vms.json`.

## 5. Labels в .container (Quadlet)

Параметры проксирования берутся из labels контейнера. В Quadlet-файл (`*.container`):

```ini
[Container]
Image=...
PublishPort=8080:8080
Label=name-app
Label=port-8080
Label=proto-http
```

| Label | Обязательность | Описание |
|---|---|---|
| `name-<имя>` | Нет | Переопределяет имя домена. По умолчанию берётся из имени контейнера с обрезкой префикса `systemd-` и нумерации (`systemd-01-athens` → `athens`). |
| `port-<порт>` | Нет | Переопределяет порт upstream. По умолчанию берётся первый `host_port` из `Ports` вывода `podman ps`. |
| `proto-<http/https>` | Нет | Протокол upstream. По умолчанию `http`. При `https` добавляется `insecure_skip_verify`. |

Если label не указан — плагин использует значения по умолчанию из `podman ps`.

## 6. Wildcard DNS в dnsmasq

Для каждой ноды, где стоит Caddy, в `/etc/dnsmasq.d/`:
```ini
address=/.yadr00.internal/172.20.5.3
address=/.yadr01.internal/172.20.6.3
```
Перезапуск dnsmasq через Intermasq (кнопка Reload).

Проверка:
```bash
dig athens.obshaga.yadr00.internal
```
Должен вернуть IP Caddy ноды (`172.20.5.3`).

## 7. Caddy (на каждой ноде)

Требования к Caddy — **те же, что в Povez**:
- Admin API на `:2019`
- Глобальная настройка Step-CA как ACME CA
- `Restart=always` в systemd unit (для "the nuke" — `/stop` + автоподъём)
- Root CA Step-CA в `/etc/caddy/root_ca.crt`

Подробности — в документации Povez, раздел «Подготовка Caddy».

## 8. Первый запуск

1. UI Pomen → «ВМ» → добавь ВМ (раздел 4).
2. «Контейнеры» → выбери ВМ → «Обновить» → увидишь список контейнеров.
3. Жми «Выдать домен» на нужном контейнере.
4. «Маршруты» → проверь, что домен появился в таблице.
5. Открой в браузере `https://<container>.<vm>.<node>.internal` — должен открыться сервис через Caddy с валидным TLS от Step-CA.

## 9. Обновление плагина

```bash
sudo systemctl stop intermasq
sudo cp pomen-linux /etc/intermasq/plugins/pomen/pomen
sudo chmod +x /etc/intermasq/plugins/pomen/pomen
sudo chown intermasq:intermasq /etc/intermasq/plugins/pomen/pomen
sudo systemctl start intermasq
```
Состояние (`vms.json`, `routes.json) сохраняется — ничего не теряется.
