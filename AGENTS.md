# AGENTS.md

Guidance for AI agents working in this repository. 

## What this project is

A terminal UI (TUI) for managing a small fleet of Linux servers over SSH. It lets the user:

- see a list of servers and their health status;
- view basic CPU/load, RAM, and disk metrics;
- list Docker containers on a selected server and their `docker stats --no-stream`;
- open an interactive SSH shell on a server;
- open a shell inside a Docker container.

Name: `pharos`. Single-binary TUI, no daemon, no server-side agents. 

## Tech stack

- **Language:** Go
- **TUI:** Charm stack — `bubbletea` (event loop/model), `bubbles` (list, viewport, table, textinput, spinner), `lipgloss` (styles)
- **SSH:** local `ssh` CLI via `os/exec` (both for non-interactive commands and interactive shells). Do **not** pull in `golang.org/x/crypto/ssh` for the MVP.
- **Config:** YAML via `gopkg.in/yaml.v3`
- **Tests:** standard `go test ./...`

## Project layout

```
cmd/pharos/main.go
internal/config/        config.go, config_test.go
internal/sshcmd/        runner.go, runner_test.go
internal/collector/     collector.go, parser.go, parser_test.go
internal/model/         model.go
internal/tui/           app.go, styles.go, keys.go, components.go
examples/config.yaml
README.md
```

## How to work

- **Iterate in small steps.** 
- **Test the parsers and config hard.** Unit-test output parsing (`docker ps`, `docker stats`, `/proc` stats), config loading/validation, and SSH arg construction. Use a **fake `Runner`** in collector tests — never hit a real server in tests.
- **Keep errors non-fatal.** SSH/Docker failures must surface in the UI, never panic or crash the app.
- **Run before declaring done:**
  ```bash
  go fmt ./...
  go test ./...
  go run ./cmd/pharos --config ./examples/config.yaml
  ```

## Key implementation rules

- **Interactive shells:** do not build a PTY inside Bubble Tea. Suspend the TUI, run `ssh` / `docker exec -it` as a foreground process with inherited stdio, then return to the TUI and refresh the selected server.
- **Container shell:** prefer `sh` over `bash` (alpine/distroless may lack bash). Optional fallback: `bash || sh`.
- **Shell-injection safety:** validate container names against `^[a-zA-Z0-9_.-]+$`; otherwise use the ID or refuse exec. Avoid shell concatenation in local args where possible.
- **Health check:** `ssh ... 'echo ok'`, 3s timeout; online iff exit 0 and stdout contains `ok`.
- **Docker JSON:** parse line-by-line; only read the minimal needed fields; missing fields render empty (Docker versions vary).
- **Docker absent / no permission:** show the error in the Docker panel; do **not** mark the server offline.
- **Config defaults:** `port` → 22, `docker` → true; `name`/`host`/`user` required; `name` unique; expand `~` in `identity_file`. No password auth — keys/ssh-agent/ssh-config only.
- **Timeouts** go through `context.Context`; include stderr in returned errors.

## Out of scope for MVP — do not build

Kubernetes, Docker Compose management, deploy/restart/start/stop of containers, password storage, web UI, background daemon, Prometheus/Grafana, historical CPU graphs, editing config from the UI, server auto-discovery, roles/users.

Post-MVP ideas (only if explicitly asked): container start/stop/restart, logs viewer, compose view, server tags/groups, parallel refresh, configurable refresh interval, alerts, encrypted secrets, Docker-over-SSH API instead of CLI.

## Definition of done

Code is `go fmt`-clean, `go test ./...` passes, the TUI launches, SSH/Docker errors are displayed (not panics), and the README lets a new user run the MVP in ~5 minutes.
