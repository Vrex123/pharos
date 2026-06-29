# pharos

A terminal UI for managing a small fleet of Linux servers over SSH. One binary,
no daemon, no server-side agents — it just drives your local `ssh` client.

`pharos` lets you:

- see your servers and their health status at a glance;
- view load average (with CPU-core context), RAM, and disk usage for a server;
- list Docker containers and their `docker stats` (CPU / memory);
- open an interactive SSH shell on a server;
- open a shell inside a Docker container;
- follow a container's logs;
- add, edit, and remove servers from the UI.

It's a lightweight alternative to a full panel like Portainer or Cockpit for
personal and small production boxes.

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

## Install

Requires Go 1.24+ and an `ssh` client on your PATH.

```bash
go install github.com/Vrex123/pharos/cmd/pharos@latest
```

Or build from a checkout:

```bash
go build -o pharos ./cmd/pharos
```

## Configuration

By default pharos reads `~/.config/pharos/config.yaml`. Override with `--config`:

```bash
pharos
pharos --config ./examples/config.yaml
```

You don't need a config file to start: if it's missing, pharos launches with an
empty fleet and you can add servers from the UI (press `a`). Added and removed
servers are saved back to the config file (the directory is created if needed).

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

**Auth:** pharos uses your existing SSH setup only — keys, `ssh-agent`, and
`~/.ssh/config`. There is no password support. Make sure you can already
`ssh user@host` non-interactively before adding a server.

## Keybindings

| Key       | Action                                    |
|-----------|-------------------------------------------|
| `↑` / `k` | move selection up                         |
| `↓` / `j` | move selection down                       |
| `tab`     | switch focus between servers / containers |
| `r`/`enter` | refresh the selected server             |
| `R`       | refresh all servers                       |
| `a`       | add a server (opens a form)               |
| `E`       | edit the selected server (opens a form)   |
| `d`       | delete the selected server (asks to confirm) |
| `s`       | open an SSH shell on the selected server  |
| `e`       | open a shell in the selected container    |
| `l`       | follow logs of the selected container     |
| `q` / `ctrl+c` | quit                                 |

`s`, `e`, and `l` suspend the TUI, run `ssh` / `docker exec -it … sh` /
`docker logs -f` as a normal foreground process, and return to the TUI
(refreshing the server) on exit. Exit a log stream with `Ctrl+C`.

### Adding / editing / removing servers

Press `a` to open the add-server form, or `E` to edit the selected server with
its current values pre-filled. Move between fields with `tab` / `↑` `↓` / `enter`,
toggle Docker with `space`, then press `enter` on the **Docker** field to save (or
`esc` to cancel). Press `d` on a selected server and confirm with `y` to remove
it. All three actions are written back to your config file immediately.

## Troubleshooting

- **Server shows `offline`.** pharos runs `ssh … 'echo ok'` with a 3s timeout.
  Confirm `ssh user@host` works from your shell with no prompts. Password-only
  servers are not supported.
- **`permission denied … Docker daemon socket`.** Your SSH user can't reach
  Docker. Add it to the `docker` group or use a user that can run `docker ps`.
  The server stays *online*; only the Docker panel shows the error.
- **Empty Docker panel / "no containers".** Either there are no running
  containers, or `docker` isn't installed on that host. Set `docker: false` for
  hosts without Docker to skip the calls.
- **Container shell won't open.** pharos uses `sh` (alpine/distroless often lack
  `bash`). Container names are validated against `^[a-zA-Z0-9_.-]+$`.

## Limitations 

pharos is intentionally small. It does **not** do Kubernetes, Docker Compose,
container start/stop/restart, password auth, a web UI, a background daemon, or
metrics history. At least for now. Container logs are streamed live (`docker logs -f`), not
searchable or persisted.

## Development

```bash
go fmt ./...
go test ./...
go run ./cmd/pharos --config ./examples/config.yaml
```
