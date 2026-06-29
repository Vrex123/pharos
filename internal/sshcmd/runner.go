// Package sshcmd runs commands on remote servers via the local `ssh` CLI.
//
// It deliberately shells out to /usr/bin/ssh (no golang.org/x/crypto/ssh) so it
// reuses the user's existing ssh config, keys, and ssh-agent. Non-interactive
// commands go through Run; interactive shells suspend the TUI and inherit stdio.
package sshcmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"

	"github.com/Vrex123/pharos/internal/model"
)

// Runner executes commands against a server.
type Runner interface {
	// Run executes command on the server non-interactively and returns stdout.
	Run(ctx context.Context, server model.Server, command string) (string, error)
	// InteractiveSSH opens a login shell on the server with inherited stdio.
	InteractiveSSH(server model.Server) error
	// InteractiveContainerShell opens a shell inside a Docker container.
	InteractiveContainerShell(server model.Server, container string) error
	// InteractiveContainerLogs follows a Docker container's logs.
	InteractiveContainerLogs(server model.Server, container string) error
}

// containerNameRe guards against shell injection in container names.
var containerNameRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// ErrInvalidContainerName is returned when a container name fails validation.
var ErrInvalidContainerName = errors.New("invalid container name")

// ExecRunner is the default Runner backed by the local ssh binary.
type ExecRunner struct{}

// New returns the default ExecRunner.
func New() *ExecRunner { return &ExecRunner{} }

// baseArgs builds the common ssh arguments for a server. extra is appended
// after the user@host target (e.g. a remote command or a -t flag goes before).
func baseArgs(server model.Server) []string {
	args := []string{
		"-p", strconv.Itoa(server.Port),
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
	}
	if server.IdentityFile != "" {
		args = append(args, "-i", server.IdentityFile)
	}
	return args
}

// target returns the user@host string.
func target(server model.Server) string {
	return server.User + "@" + server.Host
}

// runArgs builds the full ssh argument list for a non-interactive command.
func runArgs(server model.Server, command string) []string {
	args := baseArgs(server)
	args = append(args, target(server), command)
	return args
}

// Run executes command via `ssh ... user@host command`.
func (r *ExecRunner) Run(ctx context.Context, server model.Server, command string) (string, error) {
	cmd := exec.CommandContext(ctx, "ssh", runArgs(server, command)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return stdout.String(), fmt.Errorf("ssh timeout to %s", server.Name)
		}
		msg := stderr.String()
		if msg == "" {
			msg = err.Error()
		}
		return stdout.String(), fmt.Errorf("ssh %s: %s", server.Name, msg)
	}
	return stdout.String(), nil
}

// InteractiveSSH runs `ssh user@host` as a foreground process with inherited
// stdio. Caller is responsible for suspending/resuming the TUI.
func (r *ExecRunner) InteractiveSSH(server model.Server) error {
	cmd := r.SSHCommand(server)
	return cmd.Run()
}

// SSHCommand returns the interactive ssh *exec.Cmd wired to the process stdio.
// Exposed so the TUI can hand it to tea.ExecProcess.
func (r *ExecRunner) SSHCommand(server model.Server) *exec.Cmd {
	args := append(baseArgs(server), target(server))
	cmd := exec.Command("ssh", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd
}

// InteractiveContainerShell opens a shell inside a container via docker exec.
func (r *ExecRunner) InteractiveContainerShell(server model.Server, container string) error {
	cmd, err := r.ContainerShellCommand(server, container)
	if err != nil {
		return err
	}
	return cmd.Run()
}

// ContainerShellCommand returns the interactive `ssh -t ... docker exec` command.
// It prefers sh, falling back to bash. The container name is validated to
// prevent shell injection in the remote command string.
func (r *ExecRunner) ContainerShellCommand(server model.Server, container string) (*exec.Cmd, error) {
	if !containerNameRe.MatchString(container) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidContainerName, container)
	}
	// Prefer sh: alpine/distroless images often lack bash.
	remote := fmt.Sprintf("docker exec -it %s sh", container)
	args := append(baseArgs(server), "-t", target(server), remote)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd, nil
}

// InteractiveContainerLogs follows a container's logs via docker logs -f.
func (r *ExecRunner) InteractiveContainerLogs(server model.Server, container string) error {
	cmd, err := r.ContainerLogsCommand(server, container)
	if err != nil {
		return err
	}
	return cmd.Run()
}

// ContainerLogsCommand returns the interactive `ssh -t ... docker logs --tail
// 200 -f <container>` command. The container name is validated to prevent shell
// injection in the remote command string. The follow stream is exited with
// Ctrl+C.
func (r *ExecRunner) ContainerLogsCommand(server model.Server, container string) (*exec.Cmd, error) {
	if !containerNameRe.MatchString(container) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidContainerName, container)
	}
	remote := fmt.Sprintf("docker logs --tail 200 -f %s", container)
	args := append(baseArgs(server), "-t", target(server), remote)
	cmd := exec.Command("ssh", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd, nil
}

// ValidContainerName reports whether name is safe to use in a remote command.
func ValidContainerName(name string) bool {
	return containerNameRe.MatchString(name)
}
