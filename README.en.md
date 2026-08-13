<!--
Pomen - plugin for Intermasq
Copyright (C) 2026 AlexRus1234

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.
-->

[Русский](README.md) | **English** |

<div align="center">

<h1>Pomen</h1>

**Plugin for [Intermasq](../Intermasq) — assigning domain names to Podman containers**

Pomen runs as a child process of the Intermasq panel. On an administrator
request it assigns Podman containers (started via Quadlet inside virtual
machines) domain names of the form
`<container>.<vm>.<node>.internal` with HTTPS provided by Caddy. The data
source is a webhook on the virtual machine that runs `podman ps`; the Intermasq
DNS zone is not modified — Host-header routing is handled by Caddy. The plugin
is distributed as a single self-contained executable without external
dependencies.

[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue.svg?style=flat-square)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8.svg?style=flat-square)](https://go.dev/)
[![Vue](https://img.shields.io/badge/Vue-3-4FC08D.svg?style=flat-square)](https://vuejs.org/)
[![Bootstrap](https://img.shields.io/badge/Bootstrap-5-7952B3.svg?style=flat-square)](https://getbootstrap.com/)
[![Platform](https://img.shields.io/badge/Linux-any-1793D1.svg?style=flat-square)](#quick-start)

</div>

---

## Contents

- [Overview](#overview)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Relationship with the host application](#relationship-with-the-host-application)
- [Comparison with Povez](#comparison-with-povez)
- [API](#api)
- [Project layout](#project-layout)
- [Development and tests](#development-and-tests)
- [Tech stack](#tech-stack)
- [Documentation](#documentation)
- [License](#license)

> Detailed documentation for features, setup and internal architecture is in
> the [`docs/`](docs/FEATURES.md) directory. This README is an overview and a
> runbook.

## Overview

Pomen is a plugin for the [Intermasq](../Intermasq) panel. It does not run
standalone: the host launches it over a Unix socket and reverse-proxies
`/plugins/pomen/*` onto that socket **after** host-level authentication.

Its purpose is to assign domain names, on an administrator request, to Podman
containers started via Quadlet inside short-lived virtual machines. Operation:

- **On-demand polling.** No background network traffic is generated: between
  requests the plugin performs no calls. When the administrator clicks
  "Refresh", the plugin calls the virtual machine's webhook (adnanh/webhook),
  which runs `podman ps --format json` and returns the container list.
- **Caddy reverse_proxy.** A route is written to the Caddy Admin API of the
  relevant node; the TLS certificate is issued by the internal Step-CA. The
  Intermasq DNS zone is not modified — a single wildcard
  `address=/.<node>.internal/<caddy-ip>` in dnsmasq is sufficient.
- **Idempotent upsert.** Re-provisioning the same container does not create a
  duplicate TLS policy in Caddy (see [docs/INTERNALS.md](docs/INTERNALS.md)).
- **Single executable.** The Go backend and the Vue 3 / Bootstrap 5 frontend
  are combined into a single file of approximately 10 MB. The UI is embedded
  through `//go:embed web`, so no external files are required in production.

## Quick start

Pomen is a plugin. The full installation guide for an Intermasq deployment is
[docs/SETUP.md](docs/SETUP.md). Below is the short version for development.

### Requirements

- Go 1.25+
- Linux for production; Windows and macOS can be built, but the Unix socket
  only functions on POSIX systems (see `socket_unix.go` / `socket_windows.go`).

### Build

```bash
make build          # local executable ./pomen
make build-linux    # cross-compile for linux/amd64 → ./pomen-linux
```

### Production mode (under Intermasq)

The executable is placed in `/etc/intermasq/plugins/pomen/`; `manifest.json`
declares the plugin to the host. The host passes `PLUGIN_SOCKET` via an
environment variable and reverse-proxies `/plugins/pomen/*` to the socket.
See [docs/SETUP.md](docs/SETUP.md).

### Dev mode (local development without Intermasq)

```bash
cp config.example.json config.json     # edit for your environment
POMEN_DEV_PORT=18992 ./pomen           # listens on TCP :18992
curl http://127.0.0.1:18992/api/version
```

`POMEN_DEV_PORT` replaces the previous `:5001` fallback. It is now an explicit,
separate mode without a Unix socket, used only for local development and CI
smoke/E2E tests.

## Configuration

### `config.json`

Place next to the executable (or point to it via `CONFIG_FILE`). The template
is [`config.example.json`](config.example.json). Every section except
`base_domain` and `nodes` can be omitted; the defaults from
`core/constants.go` apply.

```json
{
  "base_domain": ".internal",
  "nodes": {
    "node0": { "caddy_url": "http://caddy-node0.internal:2019" }
  },
  "tls":     { "acme_ca": "https://step-ca.internal:9000/acme/acme/directory",
               "root_ca_path": "/etc/caddy/root_ca.crt" },
  "webhook": { "path": "/hooks/podman", "secret_header": "X-VM-Secret" },
  "timeouts":{ "caddy": "10s", "webhook": "15s", "restart_delay": "2s" }
}
```

### Environment variables

| Variable | Purpose | Default |
|---|---|---|
| `PLUGIN_SOCKET` | Unix socket path (set by the Intermasq host) | — (required in production) |
| `POMEN_DEV_PORT` | TCP port for dev mode (no socket) | — |
| `CONFIG_FILE` | Path to config.json | `config.json` |
| `STATE_FILE` | Path to routes.json | `/etc/intermasq/plugins/pomen/routes.json` |
| `VMS_FILE` | Path to vms.json | `/etc/intermasq/plugins/pomen/vms.json` |

See [`.env.example`](.env.example).

## Relationship with the host application

Pomen follows the Intermasq plugin contract (see `docs/func/EN/plugins.md` in
the host repository):

1. `/etc/intermasq/plugins/pomen/` contains `manifest.json` and the executable.
2. The host launches the plugin as a child process and passes `PLUGIN_SOCKET`.
3. The plugin listens on that Unix socket with mode `0770`.
4. The host mounts a reverse proxy on `/plugins/pomen/*` **after** its own
   authentication.
5. The UI mounts an `<iframe src="/plugins/pomen/">`; the frontend reads the
   JWT from `window.parent.localStorage`.

The contract does not require the plugin to authenticate on its own — that is
the host's responsibility. In dev mode (`POMEN_DEV_PORT`) the plugin runs
without authentication and without a socket.

## Comparison with Povez

Pomen and [Povez](../Intermasq-Povez) are two domain-assignment plugins that
differ in their data source:

| | Povez | Pomen |
|---|---|---|
| Source | Proxmox API | VM webhook + `podman ps` |
| Target | PVE VMs/LXC | Podman containers inside VMs |
| Parameters | PVE tags `port-`/`proto-`/`name-` | Podman labels `port-`/`proto-`/`name-` |
| Upstream IP | Computed `[subnet].[VMID-98]` | `IP_VM` (from the registry) |
| DNS in Intermasq | `dhcp-host` (DHCP reservation) | **not written** (wildcard) |
| Caddy ID | `proxy-<VMID>-<node>` | `pod-<vm>-<name>-<node>` |
| Trigger | MAC from leases | Administrator request |

Both plugins write into the same per-node Caddy instance; the `@id` prefixes
(`proxy-` and `pod-` respectively) ensure that no conflicts arise.

## API

| Method | Path | Description |
|---|---|---|
| GET | `/api/version` | Build version (JSON `{"version":"…"}`) |
| GET | `/api/nodes` | List of node keys from config.json |
| GET | `/api/vms` | VM registry (no secrets) |
| POST | `/api/vms` | Add/update a VM |
| DELETE | `/api/vms/<name>` | Remove a VM from the registry |
| GET | `/api/containers?vm=<name>` | List containers via the VM webhook |
| POST | `/api/provision` | Issue a domain to a container |
| POST | `/api/deprovision` | Remove a route by `route_id` |
| DELETE | `/api/deprovision/<route_id>` | Remove a route |
| GET | `/api/state` | Contents of routes.json |
| POST | `/api/replay` | Rewrite Caddy config from state |
| GET | `/` | UI (index.html, embedded) |
| GET | `/app.js` | UI (app.js, embedded) |

Errors: `core.ErrBadRequest` → HTTP 400, `core.ErrNotFound` → HTTP 404,
everything else → 500 (see [docs/INTERNALS.md](docs/INTERNALS.md),
“Sentinel errors” section).

## Project layout

See [README.md](README.md#структура-проекта) for the canonical Russian
version; the layout is identical.

## Development and tests

```bash
make test           # go test ./...
make test-race      # + -race (StateStore concurrency etc.)
make cover          # coverage.out + summary
make lint           # go vet + gofmt gate
make run            # build and run in dev mode
```

The full CI pipeline ([`.forgejo/workflows/build.yml`](.forgejo/workflows/build.yml)):

- **L1+L2 — Go unit** (`go test -race`)
- **L3 — bash smoke** (brings up `caddy-mock` + `webhook-mock` + `pomen-ci` in
  dev mode, runs `tests/smoke.sh`)
- **L4 — Playwright E2E** (opt-in via `run_e2e_tests=true`; one UI-mount spec)

Fixtures (`tests/fixtures/`) are separate go.mod files so that test-only code
does not pollute the main module's dependencies.

## Tech stack

- **Backend:** Go 1.25 (stdlib only — `net/http`, `encoding/json`, `log/slog`,
  `slices`, `sync`, `embed`)
- **Frontend:** Vue 3 (Composition API) + Bootstrap 5 + Axios (via CDN)
- **External services:** Caddy (Admin API + Step-CA), adnanh/webhook on VMs
- **CI/CD:** Forgejo Actions, fedora:44 runner (same as Intermasq)
- **License:** AGPL-3.0

## Documentation

- [`docs/FEATURES.md`](docs/FEATURES.md) — full feature list (RU)
- [`docs/SETUP.md`](docs/SETUP.md) — step-by-step installation (RU)
- [`docs/INTERNALS.md`](docs/INTERNALS.md) — architecture and workarounds (RU)
- [`docs/CHANGELOG.md`](docs/CHANGELOG.md) — change log (RU)
- [`SECURITY.md`](SECURITY.md) — security policy (RU+EN)

## License

[AGPL-3.0](LICENSE) © 2026 AlexRus1234
