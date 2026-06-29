# 🗼 pharos

[![CI](https://github.com/Vrex123/pharos/actions/workflows/ci.yml/badge.svg)](https://github.com/Vrex123/pharos/actions/workflows/ci.yml)
[![Release](https://github.com/Vrex123/pharos/actions/workflows/release.yml/badge.svg)](https://github.com/Vrex123/pharos/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/Vrex123/pharos)](https://goreportcard.com/report/github.com/Vrex123/pharos)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**A lightweight SSH-based terminal dashboard for Linux servers and Docker containers.**

One binary. No daemon. No server-side agents. Just your existing `ssh` setup.

```
┌ Servers ─────────────┐ ┌ Server Stats ──────────────────────────────┐
│  ● prod              │ │ host:   root@1.2.3.4:22                     │
│ ○ staging            │ │ status: online                             │
└──────────────────────┘ │ load avg: 0.18 0.10 0.05 (5%/core · 4 cores)│
                         │ ram:    1.0GB / 4.0GB (25%)                 │
                         │ disk:   42.0GB / 80.0GB (52%)               │
                         └─────────────────────────────────────────────┘
┌ Docker Containers ──────────────────────────────────────────────────┐
│ NAME              IMAGE              STATE     CPU      MEM          │
│ web               app:latest         running   2.1%     180MiB / 1G  │
│ postgres          postgres:16        running   0.5%     512MiB / 1G  │
└──────────────────────────────────────────────────────────────────────┘
↑/↓ move • tab focus • r refresh • R all • s ssh • e exec • l logs • q quit
```

## Why pharos?

Use pharos when you want a quick terminal dashboard for a few personal or small production servers without installing a web panel, daemon, monitoring stack, or server-side agent.

It is useful for:

- checking whether servers are online;
- viewing load average, RAM, and disk usage;
- seeing running Docker containers and their live CPU / memory usage;
- jumping into SSH shells, container shells, or container logs quickly;
- keeping a small fleet in a simple YAML config that can also be edited from the UI.

pharos is a lightweight alternative to a full panel like Portainer or Cockpit when all you need is a fast terminal view over SSH.

## Features

- **Server overview:** online/offline status, load average with CPU-core context, RAM usage, and disk usage.
- **Docker view:** running containers with `docker stats --no-stream` CPU and memory data.
- **Interactive access:** open SSH shells, container shells, and live container logs from the TUI.
- **Configurable fleet:** add, edit, and remove servers without leaving pharos.
- **Zero agent setup:** uses your local `ssh` client, keys, `ssh-agent`, and `~/.ssh/config`.
- **Single binary:** release builds are produced by CI for Linux, macOS, and Windows.

## Requirements

On your local machine:

- an `ssh` client available on your `PATH`;
- working key-based SSH access to your servers;
- Go 1.24+ only if you install from source.

On remote servers:

- Linux shell access;
- Docker CLI access if `docker: true` is enabled for that server.

pharos does not support password authentication. Make sure you can already run `ssh user@host` non-interactively before adding a server.

## Install

pharos is distributed as a single binary. Download the latest archive for your OS and architecture from the [GitHub Releases](https://github.com/Vrex123/pharos/releases) page.

Release builds are available for Linux, macOS, and Windows. Archives include the `pharos` binary, this README, and `examples/config.yaml`. Checksums are published as `checksums.txt` with every release.

### Linux / macOS

Download the archive for your OS and architecture from [Releases](https://github.com/Vrex123/pharos/releases), then:

```bash
tar -xzf pharos_*.tar.gz
chmod +x pharos
sudo mv pharos /usr/local/bin/
pharos --version
```

### Windows

Download the `.zip` archive from [Releases](https://github.com/Vrex123/pharos/releases), extract `pharos.exe`, and place it somewhere on your `PATH`.

### Install with Go

Requires Go 1.24+:

```bash
go install github.com/Vrex123/pharos/cmd/pharos@latest
```

### Build from source

```bash
git clone https://github.com/Vrex123/pharos.git
cd pharos
go build -o pharos ./cmd/pharos
```

## Quick start

Start pharos:

```bash
pharos
```

By default pharos reads `~/.config/pharos/config.yaml`. You do not need to create it manually: if the file is missing, pharos launches with an empty fleet and you can press `a` to add your first server.

To start with a specific config file:

```bash
pharos --config ./examples/config.yaml
```

## Configuration

Example config (see [`examples/config.yaml`](./examples/config.yaml)):

```yaml
servers:
  - name: prod
    host: 1.2.3.4
    port: 22
    user: root
    docker: true

  - name: staging
    host: staging.example.com
    port: 22
    user: deploy
    identity_file: ~/.ssh/id_ed25519
    docker: true
```

Rules:

- `name` — required, unique.
- `host` — required.
- `user` — required.
- `port` — optional, default `22`.
- `docker` — optional, default `true`.
- `identity_file` — optional; a leading `~` is expanded.

Added, edited, and removed servers are saved back to the config file immediately. The config directory is created automatically if needed.

**Auth:** pharos uses your existing SSH setup only — keys, `ssh-agent`, and `~/.ssh/config`. There is no password support. Make sure you can already `ssh user@host` non-interactively before adding a server.

## Keybindings

| Key       | Action                                    |
|-----------|-------------------------------------------|
| `↑` / `k` | move selection up                         |
| `↓` / `j` | move selection down                       |
| `tab`     | switch focus between servers / containers |
| `r` / `enter` | refresh the selected server          |
| `R`       | refresh all servers                       |
| `a`       | add a server (opens a form)               |
| `E`       | edit the selected server (opens a form)   |
| `d`       | delete the selected server (asks to confirm) |
| `s`       | open an SSH shell on the selected server  |
| `e`       | open a shell in the selected container    |
| `l`       | follow logs of the selected container     |
| `q` / `ctrl+c` | quit                                 |

`s`, `e`, and `l` suspend the TUI, run `ssh` / `docker exec -it … sh` / `docker logs -f` as a normal foreground process, and return to the TUI, refreshing the server on exit. Exit a log stream with `Ctrl+C`.

### Adding, editing, and removing servers

Press `a` to open the add-server form, or `E` to edit the selected server with its current values pre-filled. Move between fields with `tab` / `↑` / `↓` / `enter`, toggle Docker with `space`, then press `enter` on the **Docker** field to save, or `esc` to cancel.

Press `d` on a selected server and confirm with `y` to remove it.

## Troubleshooting

- **Server shows `offline`.** pharos runs `ssh … 'echo ok'` with a 3s timeout. Confirm `ssh user@host` works from your shell with no prompts. Password-only servers are not supported.
- **`permission denied … Docker daemon socket`.** Your SSH user cannot reach Docker. Add it to the `docker` group or use a user that can run `docker ps`. The server stays *online*; only the Docker panel shows the error.
- **Empty Docker panel / "no containers".** Either there are no running containers, or `docker` is not installed on that host. Set `docker: false` for hosts without Docker to skip the calls.
- **Container shell will not open.** pharos uses `sh` because Alpine-based images often do not include `bash`. Container names are validated against `^[a-zA-Z0-9_.-]+$`.

## What pharos is not

pharos is intentionally small. It is not a replacement for Kubernetes, Portainer, Cockpit, Prometheus, Grafana, or a full monitoring platform.

It does not currently provide:

- Kubernetes or Docker Compose management;
- container start/stop/restart actions;
- password authentication;
- a web UI or background daemon;
- metrics history, alerting, or log persistence.

At least for now.

## Development

```bash
go fmt ./...
go vet ./...
go test ./...
go run ./cmd/pharos --config ./examples/config.yaml
```

## License

pharos is released under the [MIT License](./LICENSE).
