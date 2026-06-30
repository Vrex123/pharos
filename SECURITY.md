# Security Policy

pharos connects to servers through your local `ssh` client and uses your existing SSH configuration, keys, and `ssh-agent`. It does not run a server-side agent and does not intentionally collect or store credentials.

## Supported versions

Security fixes are handled on the default branch and in the latest released version of pharos. Please use the newest release from [GitHub Releases](https://github.com/Vrex123/pharos/releases) when possible.

## Reporting a vulnerability

Please do **not** report security vulnerabilities through public GitHub issues.

If you believe you have found a vulnerability, report it privately using GitHub's private vulnerability reporting if it is enabled for this repository, or contact the repository owner through GitHub with enough information to reproduce and assess the issue.

Helpful details include:

- pharos version or commit;
- operating system and terminal;
- installation method;
- affected command, config, or workflow;
- steps to reproduce;
- impact and any known workaround;
- redacted logs or screenshots if useful.

Please do not include private keys, real hostnames, IP addresses, passwords, tokens, or other secrets in the report unless explicitly requested through a private channel.

## Scope

Security-sensitive areas include:

- SSH command argument construction;
- container shell/log command construction;
- config file handling and path expansion;
- terminal escape/control sequence handling;
- accidental disclosure of host, user, or command output data.

## Expectations

Maintainers will acknowledge valid reports when they are able to access them, investigate the issue, and coordinate a fix or mitigation before public disclosure where appropriate.

## User security notes

- pharos relies on your local SSH setup. Keep private keys protected and prefer `ssh-agent` where appropriate.
- Avoid putting sensitive production details in shared screenshots or public issue reports.
- pharos does not support password authentication and should not be modified to store server passwords.
