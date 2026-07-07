# Pomen

**Pomen** — подключаемый модуль для [Intermasq](https://github.com/your-repo/intermasq). 
Предназначен для автоматической выдачи доменных имен контейнерам Podman, запущенным через Quadlet внутри виртуальных машин.

## Кратко

- **Цель:** по кнопке в UI выдать контейнеру домен вида `name.vm.node.internal` с HTTPS через Caddy.
- **Источник данных:** вебхук на ВМ (adnanh/webhook), исполняющий `podman ps --format json` по запросу плагина.
- **DNS:** ядро Intermasq **не трогается** — в dnsmasq прописывается wildcard `address=/.node.internal/CADDY_IP`, маршрутизация по Host-header делает Caddy.
- **Caddy:** используются те же инстансы Caddy, что и в Povez (per-нода). Маршруты Pomen имеют префикс `pod-` и не конфликтуют с `proxy-` из Povez.
- **Язык:** Go (бэкенд) + Vue 3 (фронтенд, один HTML-файл). Поставляется одним бинарем.

## Связь с другими плагинами

| | Povez (yadr-prov) | Pomen |
|---|---|---|
| Источник | Proxmox API | Вебхук ВМ + `podman ps` |
| Объект | ВМ/LXC PVE | Контейнеры Podman внутри ВМ |
| Параметры | Теги PVE `port-`/`proto-`/`name-` | Labels Podman `port-`/`proto-`/`name-` (или поля `Ports`) |
| IP upstream | Вычисление `[subnet].[VMID-98]` | `IP_VM` (из реестра) |
| DNS в Intermasq | `dhcp-host` (DHCP-резервация) | **не пишется** (wildcard) |
| Caddy ID | `proxy-<VMID>-<node>` | `pod-<vm>-<name>-<node>` |
| Trigger | По MAC из leases | По кнопке в UI |

## Лицензия

Apache 2.0
