# Contributing to pharos

Thanks for taking the time to improve pharos 🗼

pharos is a small, single-binary terminal UI for checking Linux servers over SSH. Contributions are welcome, especially fixes that keep the app simple, reliable, and easy to run without a daemon or server-side agent.

## Before you start

Please keep the project goals in mind:

- use the local `ssh` CLI and existing SSH keys/config;
- keep remote servers agentless;
- surface SSH and Docker errors in the UI instead of crashing;
- prefer small, focused changes over broad rewrites;
- keep the README and examples accurate when behavior changes.

For implementation guidance, see [`AGENTS.md`](./AGENTS.md). It documents the current architecture, project layout, and important constraints.

## Development setup

Requirements:

- Go 1.24+;
- an `ssh` client on your `PATH`;
- Docker only if you want to test Docker-related behavior against real hosts.

Clone and test:

```bash
git clone https://github.com/Vrex123/pharos.git
cd pharos
go test ./...
```

Run locally:

```bash
go run ./cmd/pharos --config ./examples/config.yaml
```

Or build a local binary:

```bash
go build -o pharos ./cmd/pharos
./pharos --config ./examples/config.yaml
```

## Working on changes

1. Create a branch from `main`.
2. Keep the change focused on one bug, feature, or documentation improvement.
3. Add or update tests for parser, config, SSH argument, and UI-state behavior where practical.
4. Run formatting and tests before opening a pull request:

```bash
go fmt ./...
go vet ./...
go test ./...
```

If you change user-facing behavior, update [`README.md`](./README.md) and [`examples/config.yaml`](./examples/config.yaml) if needed.

## Testing expectations

Please avoid tests that require a real remote server. Prefer fake runners and sample command output.

Good areas for tests:

- config loading, validation, defaults, and `~` expansion;
- SSH argument construction;
- Docker command output parsing;
- process list parsing;
- non-fatal error handling in collectors and TUI state.

## Pull request checklist

Before opening a PR, please confirm:

- [ ] `go fmt ./...` has been run;
- [ ] `go vet ./...` passes;
- [ ] `go test ./...` passes;
- [ ] documentation/examples are updated if needed;
- [ ] the change does not add a background daemon or server-side agent;
- [ ] SSH/Docker failures remain visible to users and do not panic the app.

## Reporting bugs

When filing a bug, include:

- pharos version or commit;
- OS and terminal emulator;
- how you installed pharos;
- minimal config with secrets/hosts anonymized;
- exact steps to reproduce;
- relevant error text from the UI or terminal.

Never paste private SSH keys, real credentials, or sensitive server details into public issues.

## Security issues

Please do not open public issues for security vulnerabilities. Follow [`SECURITY.md`](./SECURITY.md) instead.
